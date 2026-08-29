# Auctioneer

Go auction engine backed by Redis. Bids are applied atomically with a Lua script (idempotent on `bid_id`). Accepted bids are written to a Redis stream and fanned out over WebSockets.

## Run

Redis on `127.0.0.1:6379`, then:

```bash
pnpm --filter api dev
```

From the repo root, `pnpm dev:api` does the same. `pnpm dev` also starts the dashboard.

| Endpoint | Port |
|---|---|
| gRPC `CreateAuction` / `PlaceBid` | `:8081` |
| `GET /healthz` | `:8080` |
| `GET /readyz` | `:8080` (Redis must be up) |
| `GET /metrics` | `:8080` |
| `GET /ws?auction_id=<id>` | `:8080` |

`PlaceBid` reasons include `accepted`, `bid_too_low`, `auction_ended`, `auction_closed`, and `auction_not_found`. Duplicate `bid_id`s return the cached result with `duplicate=true`.

## Test

```bash
pnpm --filter api test
```

Unit tests use miniredis. A live Redis instance is only required for integration tests, benchmarks, and load:

```bash
REDIS_ADDR=127.0.0.1:6379 pnpm --filter api bench
pnpm --filter api load          # API must already be running
pnpm --filter api test:all      # starts Redis + API, then test/bench/load
```

Load defaults: 1,000,000 bids, 512 workers, `127.0.0.1:8081`.

## Config

| Variable | Default |
|---|---|
| `AUCTION_GRPC_ADDR` | `:8081` |
| `AUCTION_HTTP_ADDR` | `:8080` |
| `REDIS_ADDRS` | `127.0.0.1:6379` |
| `REDIS_POOL_SIZE` | `256` |
| `MAX_INFLIGHT_BIDS` | `100000` |
| `MAX_WS_QUEUE_DEPTH` | `2048` |
| `DEFAULT_AUCTION_DURATION` | `10m` |
| `STREAM_READ_COUNT` | `256` |
| `BID_IDEMPOTENCY_TTL` | `10m` |
| `BREAKER_FAILURE_THRESHOLD` | `20` |
| `BREAKER_OPEN_FOR` | `500ms` |

If Redis is unhealthy, the breaker fails bid writes fast. WebSocket connections stay up; clients dropped under backpressure are counted on `auction_ws_dropped_clients_total`.
