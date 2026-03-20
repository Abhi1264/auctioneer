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

local cached = redis.call("HGET", idemKey, bidId)
if cached then
  return {"DUP", cached}
end

local status = redis.call("HGET", stateKey, "status")
if not status then
  return {"ERR", "auction_not_found", "0", "", "0", "", tostring(nowMs)}
end

if status ~= "open" then
  local current = tonumber(redis.call("HGET", stateKey, "current_price") or "0")
  local winner = redis.call("HGET", stateKey, "winner_user_id") or ""
  local version = tonumber(redis.call("HGET", stateKey, "version") or "0")
  local encoded = "0|auction_closed|" .. tostring(current) .. "|" .. winner .. "|" .. tostring(version) .. "||" .. tostring(nowMs)
  redis.call("HSET", idemKey, bidId, encoded)
  redis.call("PEXPIRE", idemKey, idemTTL)
  return {"OK", "0", "auction_closed", tostring(current), winner, tostring(version), "", tostring(nowMs)}
end

local endAt = tonumber(redis.call("HGET", stateKey, "end_at") or "0")
if nowMs >= endAt then
  redis.call("HSET", stateKey, "status", "closed")
  local current = tonumber(redis.call("HGET", stateKey, "current_price") or "0")
  local winner = redis.call("HGET", stateKey, "winner_user_id") or ""
  local version = tonumber(redis.call("HGET", stateKey, "version") or "0")
  local encoded = "0|auction_ended|" .. tostring(current) .. "|" .. winner .. "|" .. tostring(version) .. "||" .. tostring(nowMs)
  redis.call("HSET", idemKey, bidId, encoded)
  redis.call("PEXPIRE", idemKey, idemTTL)
  return {"OK", "0", "auction_ended", tostring(current), winner, tostring(version), "", tostring(nowMs)}
end

local currentPrice = tonumber(redis.call("HGET", stateKey, "current_price") or "0")
local winner = redis.call("HGET", stateKey, "winner_user_id") or ""
local version = tonumber(redis.call("HGET", stateKey, "version") or "0")

if amount <= currentPrice then
  local encoded = "0|bid_too_low|" .. tostring(currentPrice) .. "|" .. winner .. "|" .. tostring(version) .. "||" .. tostring(nowMs)
  redis.call("HSET", idemKey, bidId, encoded)
  redis.call("PEXPIRE", idemKey, idemTTL)
  return {"OK", "0", "bid_too_low", tostring(currentPrice), winner, tostring(version), "", tostring(nowMs)}
end

version = version + 1
redis.call("HSET", stateKey, "current_price", tostring(amount), "winner_user_id", userId, "version", tostring(version))
local eventId = redis.call("XADD", eventsKey, "*",
  "auction_id", redis.call("HGET", stateKey, "auction_id"),
  "bid_id", bidId,
  "user_id", userId,
  "amount", tostring(amount),
  "version", tostring(version),
  "server_ts", tostring(nowMs))

local encoded = "1|accepted|" .. tostring(amount) .. "|" .. userId .. "|" .. tostring(version) .. "|" .. eventId .. "|" .. tostring(nowMs)
redis.call("HSET", idemKey, bidId, encoded)
redis.call("PEXPIRE", idemKey, idemTTL)
return {"OK", "1", "accepted", tostring(amount), userId, tostring(version), eventId, tostring(nowMs)}
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

func (r *RedisRouter) ForAuction(auctionID string) *redis.Client {
	if len(r.clients) == 1 {
		return r.clients[0]
	}
	idx := int(crc32.ChecksumIEEE([]byte(auctionID)) % uint32(len(r.clients)))
	return r.clients[idx]
}

func (r *RedisRouter) Clients() []*redis.Client {
	return r.clients
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
	router      *RedisRouter
	idemTTL     time.Duration
	streamCount int64
	script      *redis.Script
	breaker     *circuitBreaker
}

func NewRedisStore(router *RedisRouter, cfg config.Config) *RedisStore {
	return &RedisStore{
		router:      router,
		idemTTL:     cfg.BidIdempotencyTTL,
		streamCount: cfg.StreamReadCount,
		script:      redis.NewScript(bidScript),
		breaker:     newCircuitBreaker(cfg.BreakerFailureThreshold, cfg.BreakerOpenFor),
	}
}

func (s *RedisStore) CreateAuction(ctx context.Context, req CreateAuctionRequest) error {
	stateKey := stateKey(req.AuctionID)
	client := s.router.ForAuction(req.AuctionID)
	pipe := client.Pipeline()
	pipe.HSet(ctx, stateKey,
		"auction_id", req.AuctionID,
		"status", "open",
		"current_price", req.OpeningPriceCents,
		"winner_user_id", "",
		"version", 0,
		"end_at", req.EndAt.UnixMilli(),
	)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) PlaceBid(ctx context.Context, req PlaceBidRequest) (PlaceBidResult, error) {
	if !s.breaker.allow() {
		return PlaceBidResult{}, ErrRedisBreakerOpen
	}
	client := s.router.ForAuction(req.AuctionID)
	nowMs := time.Now().UnixMilli()
	resp, err := s.script.Run(ctx, client, []string{
		stateKey(req.AuctionID),
		idemKey(req.AuctionID),
		eventsKey(req.AuctionID),
	}, req.BidID, req.UserID, req.AmountCents, nowMs, s.idemTTL.Milliseconds()).Result()
	if err != nil {
		s.breaker.fail()
		return PlaceBidResult{}, err
	}
	values, ok := resp.([]interface{})
	if !ok || len(values) < 2 {
		return PlaceBidResult{}, fmt.Errorf("unexpected script response")
	}

	tag := strVal(values[0])
	var result PlaceBidResult
	if tag == "DUP" {
		result = decodeCachedResult(strVal(values[1]))
		result.Duplicate = true
	} else {
		if len(values) < 8 {
			return PlaceBidResult{}, fmt.Errorf("unexpected script response")
		}
		accepted := strVal(values[1]) == "1"
		result = PlaceBidResult{
			Accepted:        accepted,
			Reason:          strVal(values[2]),
			CurrentPrice:    int64Val(values[3]),
			WinnerUserID:    strVal(values[4]),
			Version:         int64Val(values[5]),
			EventID:         strVal(values[6]),
			ServerUnixMilli: int64Val(values[7]),
		}
	}
	if tag == "ERR" {
		s.breaker.fail()
		return result, errors.New(result.Reason)
	}
	s.breaker.success()

	if result.Accepted {
		pubChannel := bidsChannel(req.AuctionID)
		payload := fmt.Sprintf("%s|%s|%s|%d|%d|%d", req.AuctionID, req.BidID, req.UserID, req.AmountCents, result.Version, result.ServerUnixMilli)
		_ = client.Publish(ctx, pubChannel, payload).Err()
	}
	return result, nil
}

func (s *RedisStore) BreakerOpen() bool {
	return s.breaker.isOpen()
}

func (s *RedisStore) SubscribeAuction(ctx context.Context, auctionID string, handler func(AuctionEvent)) error {
	client := s.router.ForAuction(auctionID)
	sub := client.Subscribe(ctx, bidsChannel(auctionID))
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			parts := strings.Split(msg.Payload, "|")
			if len(parts) < 6 {
				continue
			}
			handler(AuctionEvent{
				AuctionID:       parts[0],
				BidID:           parts[1],
				UserID:          parts[2],
				AmountCents:     parseInt64(parts[3]),
				Version:         parseInt64(parts[4]),
				ServerUnixMilli: parseInt64(parts[5]),
			})
		}
	}
}

func (s *RedisStore) ReadEvents(ctx context.Context, offsets map[string]string, count int64) ([]AuctionEvent, error) {
	out := make([]AuctionEvent, 0, count)
	for auctionID, offset := range offsets {
		client := s.router.ForAuction(auctionID)
		if offset == "" {
			offset = "0-0"
		}
		streams, err := client.XRead(ctx, &redis.XReadArgs{
			Streams: []string{eventsKey(auctionID), offset},
			Count:   count,
			Block:   5 * time.Millisecond,
		}).Result()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				out = append(out, AuctionEvent{
					ID:              msg.ID,
					AuctionID:       val(msg.Values, "auction_id"),
					BidID:           val(msg.Values, "bid_id"),
					UserID:          val(msg.Values, "user_id"),
					AmountCents:     parseInt64(val(msg.Values, "amount")),
					Version:         parseInt64(val(msg.Values, "version")),
					ServerUnixMilli: parseInt64(val(msg.Values, "server_ts")),
				})
			}
		}
	}
	return out, nil
}

func stateKey(auctionID string) string  { return fmt.Sprintf("auc:{%s}:state", auctionID) }
func idemKey(auctionID string) string   { return fmt.Sprintf("auc:{%s}:idem", auctionID) }
func eventsKey(auctionID string) string { return fmt.Sprintf("auc:{%s}:events", auctionID) }
func bidsChannel(auctionID string) string {
	return fmt.Sprintf("auc:{%s}:pubsub", auctionID)
}

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

func int64Val(v interface{}) int64 { return parseInt64(strVal(v)) }

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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
