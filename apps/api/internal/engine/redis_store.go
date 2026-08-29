package engine

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/config"
	"github.com/redis/go-redis/v9"
)

const bidScript = `
local stateKey = KEYS[1]
local idemKey = KEYS[2]
local eventsKey = KEYS[3]

local bidId = ARGV[1]
local userId = ARGV[2]
local amount = tonumber(ARGV[3])
local nowMs = tonumber(ARGV[4])
local idemTTL = tonumber(ARGV[5])
local auctionId = ARGV[6]

local function encode(accepted, reason, price, winner, version, eventId)
  return accepted .. "|" .. reason .. "|" .. tostring(price) .. "|" .. winner .. "|" .. tostring(version) .. "|" .. eventId .. "|" .. tostring(nowMs)
end

local function cache(encoded)
  redis.call("HSET", idemKey, bidId, encoded)
  redis.call("PEXPIRE", idemKey, idemTTL)
end

local cached = redis.call("HGET", idemKey, bidId)
if cached then
  return {"DUP", cached}
end

local fields = redis.call("HMGET", stateKey, "status", "end_at", "current_price", "winner_user_id", "version")
local status = fields[1]
if not status then
  return {"ERR", encode("0", "auction_not_found", 0, "", 0, "")}
end

local endAt = tonumber(fields[2] or "0")
local currentPrice = tonumber(fields[3] or "0")
local winner = fields[4] or ""
local version = tonumber(fields[5] or "0")

if status ~= "open" then
  local encoded = encode("0", "auction_closed", currentPrice, winner, version, "")
  cache(encoded)
  return {"OK", encoded}
end

if nowMs >= endAt then
  redis.call("HSET", stateKey, "status", "closed")
  local encoded = encode("0", "auction_ended", currentPrice, winner, version, "")
  cache(encoded)
  return {"OK", encoded}
end

if amount <= currentPrice then
  local encoded = encode("0", "bid_too_low", currentPrice, winner, version, "")
  cache(encoded)
  return {"OK", encoded}
end

version = version + 1
redis.call("HSET", stateKey, "current_price", tostring(amount), "winner_user_id", userId, "version", tostring(version))
local eventId = redis.call("XADD", eventsKey, "*",
  "auction_id", auctionId,
  "bid_id", bidId,
  "user_id", userId,
  "amount", tostring(amount),
  "version", tostring(version),
  "server_ts", tostring(nowMs))

local encoded = encode("1", "accepted", amount, userId, version, eventId)
cache(encoded)
return {"OK", encoded}
`

type RedisRouter struct {
	clients []*redis.Client
	closed  atomic.Bool
}

func NewRedisRouter(addresses []string, password string, db int, poolSize int) (*RedisRouter, error) {
	clients := make([]*redis.Client, 0, len(addresses))
	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		client := redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     password,
			DB:           db,
			PoolSize:     poolSize,
			MinIdleConns: max(poolSize/8, 8),
			ReadTimeout:  100 * time.Millisecond,
			WriteTimeout: 100 * time.Millisecond,
			DialTimeout:  200 * time.Millisecond,
		})
		clients = append(clients, client)
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("no redis clients configured")
	}
	return &RedisRouter{clients: clients}, nil
}

func (r *RedisRouter) shard(auctionID string) int {
	n := len(r.clients)
	if n == 1 {
		return 0
	}
	return int(crc32.ChecksumIEEE([]byte(auctionID)) % uint32(n))
}

func (r *RedisRouter) ForAuction(auctionID string) *redis.Client {
	return r.clients[r.shard(auctionID)]
}

func (r *RedisRouter) Close() {
	if !r.closed.CompareAndSwap(false, true) {
		return
	}
	for _, c := range r.clients {
		_ = c.Close()
	}
}

type RedisStore struct {
	router  *RedisRouter
	idemTTL time.Duration
	script  *redis.Script
	breaker *circuitBreaker
}

func NewRedisStore(router *RedisRouter, cfg config.Config) *RedisStore {
	return &RedisStore{
		router:  router,
		idemTTL: cfg.BidIdempotencyTTL,
		script:  redis.NewScript(bidScript),
		breaker: newCircuitBreaker(cfg.BreakerFailureThreshold, cfg.BreakerOpenFor),
	}
}

func (s *RedisStore) CreateAuction(ctx context.Context, req CreateAuctionRequest) error {
	return s.router.ForAuction(req.AuctionID).HSet(ctx, stateKey(req.AuctionID),
		"auction_id", req.AuctionID,
		"status", "open",
		"current_price", req.OpeningPriceCents,
		"winner_user_id", "",
		"version", 0,
		"end_at", req.EndAt.UnixMilli(),
	).Err()
}

func (s *RedisStore) PlaceBid(ctx context.Context, req PlaceBidRequest) (PlaceBidResult, error) {
	if !s.breaker.allow() {
		return PlaceBidResult{}, ErrRedisBreakerOpen
	}
	resp, err := s.script.Run(ctx, s.router.ForAuction(req.AuctionID), []string{
		stateKey(req.AuctionID),
		idemKey(req.AuctionID),
		eventsKey(req.AuctionID),
	}, req.BidID, req.UserID, req.AmountCents, time.Now().UnixMilli(), s.idemTTL.Milliseconds(), req.AuctionID).Result()
	if err != nil {
		s.breaker.fail()
		return PlaceBidResult{}, err
	}
	values, ok := resp.([]interface{})
	if !ok || len(values) < 2 {
		return PlaceBidResult{}, fmt.Errorf("unexpected script response")
	}

	result := decodeCachedResult(strVal(values[1]))
	switch strVal(values[0]) {
	case "DUP":
		result.Duplicate = true
		s.breaker.success()
		return result, nil
	case "ERR":
		return result, errors.New(result.Reason)
	case "OK":
		s.breaker.success()
		return result, nil
	default:
		s.breaker.fail()
		return PlaceBidResult{}, fmt.Errorf("unexpected script response")
	}
}

func (s *RedisStore) BreakerOpen() bool {
	return s.breaker.isOpen()
}

func (s *RedisStore) ReadEvents(ctx context.Context, offsets map[string]string, count int64) ([]AuctionEvent, error) {
	if len(offsets) == 0 {
		return nil, nil
	}

	type shardRead struct {
		keys []string
		ids  []string
	}
	shards := make([]shardRead, len(s.router.clients))
	for auctionID, offset := range offsets {
		if offset == "" {
			offset = "0-0"
		}
		i := s.router.shard(auctionID)
		shards[i].keys = append(shards[i].keys, eventsKey(auctionID))
		shards[i].ids = append(shards[i].ids, offset)
	}

	out := make([]AuctionEvent, 0, int(count))
	last := -1
	for i, shard := range shards {
		if len(shard.keys) > 0 {
			last = i
		}
	}
	for i, shard := range shards {
		if len(shard.keys) == 0 {
			continue
		}
		streams := make([]string, 0, len(shard.keys)*2)
		streams = append(streams, shard.keys...)
		streams = append(streams, shard.ids...)
		block := time.Duration(-1)
		if i == last {
			block = 5 * time.Millisecond
		}
		res, err := s.router.clients[i].XRead(ctx, &redis.XReadArgs{
			Streams: streams,
			Count:   count,
			Block:   block,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, err
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				out = append(out, eventFromStream(msg))
			}
		}
	}
	return out, nil
}

func stateKey(auctionID string) string  { return "auc:{" + auctionID + "}:state" }
func idemKey(auctionID string) string   { return "auc:{" + auctionID + "}:idem" }
func eventsKey(auctionID string) string { return "auc:{" + auctionID + "}:events" }

func strVal(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func decodeCachedResult(encoded string) PlaceBidResult {
	parts := strings.SplitN(encoded, "|", 7)
	for len(parts) < 7 {
		parts = append(parts, "")
	}
	return PlaceBidResult{
		Accepted:        parts[0] == "1",
		Reason:          parts[1],
		CurrentPrice:    parseInt64(parts[2]),
		WinnerUserID:    parts[3],
		Version:         parseInt64(parts[4]),
		EventID:         parts[5],
		ServerUnixMilli: parseInt64(parts[6]),
	}
}

func eventFromStream(msg redis.XMessage) AuctionEvent {
	return AuctionEvent{
		ID:              msg.ID,
		AuctionID:       val(msg.Values, "auction_id"),
		BidID:           val(msg.Values, "bid_id"),
		UserID:          val(msg.Values, "user_id"),
		AmountCents:     parseInt64(val(msg.Values, "amount")),
		Version:         parseInt64(val(msg.Values, "version")),
		ServerUnixMilli: parseInt64(val(msg.Values, "server_ts")),
	}
}

func parseInt64(v string) int64 {
	out, _ := strconv.ParseInt(v, 10, 64)
	return out
}

func val(m map[string]interface{}, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	return strVal(raw)
}
