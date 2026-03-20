# Auction API

High-throughput auction engine in Go + Redis with:
- gRPC bid ingest (`/auction.v1.AuctionService/PlaceBid`)
- WebSocket fanout (`/ws?auction_id=<id>`)
- Durable event stream (`auc:{auctionId}:events`)
- Idempotent bid processing via Redis Lua

## Local Run

1. Start Redis (`127.0.0.1:6379` by default).
2. Run API:

```bash
pnpm --filter api dev
```

3. Health and metrics:
   - `GET /healthz`
   - `GET /readyz`
   - `GET /metrics`

## Load Test

```bash
pnpm --filter api load
```

The load runner sends 1,000,000 bids by default and prints total successes, errors, elapsed time, and throughput.

## Benchmark

```bash
REDIS_ADDR=127.0.0.1:6379 pnpm --filter api bench
```

## Key Config

- `AUCTION_GRPC_ADDR` (default `:8081`)
- `AUCTION_HTTP_ADDR` (default `:8080`)
- `REDIS_ADDRS` (comma-separated, default `127.0.0.1:6379`)
- `REDIS_POOL_SIZE` (default `256`)
- `MAX_INFLIGHT_BIDS` (default `100000`)
- `MAX_WS_QUEUE_DEPTH` (default `2048`)
- `BID_IDEMPOTENCY_TTL` (default `10m`)
- `BREAKER_FAILURE_THRESHOLD` (default `20`)
- `BREAKER_OPEN_FOR` (default `500ms`)
- `GOGC` (default `100`)
- `GOMEMLIMIT_BYTES` (default `0`, disabled)

## Performance Tuning Checklist

- Increase `REDIS_POOL_SIZE` until Redis latency no longer decreases.
- Keep `MAX_INFLIGHT_BIDS` bounded to avoid scheduler and queue collapse.
- Tune `GOGC` and `GOMEMLIMIT_BYTES` together; watch p99 latency, RSS, and GC pauses.
- Monitor:
  - `auction_bid_requests_total`
  - `auction_bid_latency_seconds`
  - `auction_inflight_bids`
  - `auction_redis_breaker_open`
  - `auction_ws_dropped_clients_total`
  - `auction_stream_dispatch_lag_ms`

## Failure Recovery

- If Redis breaker is open:
  - Fail fast on bid writes.
  - Keep reads and WS connections alive where possible.
  - Recover by restoring Redis health and observing breaker close.
- If WS misses events:
  - Backfill from Streams using last known stream ID.
- During shard migration:
  - Use dual-write and verify parity before cutover.
  - Keep rollback path to previous shard routing map.
