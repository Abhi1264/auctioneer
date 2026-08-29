# Auction API

High-throughput auction engine in Go + Redis:

- gRPC bid ingest (`PlaceBid` / `CreateAuction`)
- WebSocket fanout (`/ws?auction_id=<id>`)
- Durable event stream (`auc:{auctionId}:events`)
- Idempotent bid processing via Redis Lua

## Local run

1. Start Redis on `127.0.0.1:6379`.
2. Start the API:

```bash
pnpm --filter api dev
```

Or from the repo root: `pnpm dev:api`.

3. Check:
   - `GET /healthz`
   - `GET /readyz` (fails until Redis is up)
   - `GET /metrics`

`pnpm dev` also starts the dashboard and CLI stubs. Use `dev:api` if you only want the server.

## Load test

Needs Redis and the API already running:

```bash
pnpm --filter api load
```

Defaults: 1,000,000 bids, 512 workers, `127.0.0.1:8081`. Override with flags on `go run ./bench/loadtest`.

To start Redis, the API, wait for `/readyz`, then run tests, bench, and load in one shot:

```bash
pnpm --filter api test:all
```

## Benchmark

```bash
REDIS_ADDR=127.0.0.1:6379 pnpm --filter api bench
```

`REDIS_ADDRS` (comma-separated) is also accepted.

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

When the Redis breaker is open, bid writes fail fast. WebSocket connections stay up. Dropped clients are counted on `auction_ws_dropped_clients_total`.
