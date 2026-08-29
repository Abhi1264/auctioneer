#!/usr/bin/env bash
set -euo pipefail

API_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REDIS_PORT="${REDIS_PORT:-6379}"
GRPC_ADDR="${GRPC_ADDR:-127.0.0.1:8081}"
HTTP_ADDR="${HTTP_ADDR:-127.0.0.1:8080}"
REQUESTS="${REQUESTS:-1000000}"
CONCURRENCY="${CONCURRENCY:-512}"
RPC_TIMEOUT_MS="${RPC_TIMEOUT_MS:-500}"
SOAK_RUNS="${SOAK_RUNS:-3}"
REDIS_ADDR="${REDIS_ADDR:-${REDIS_ADDRS:-127.0.0.1:6379}}"
REDIS_ADDR="${REDIS_ADDR%%,*}"

SERVER_PID=""
REDIS_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${REDIS_PID}" ]]; then
    kill "${REDIS_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_for_http() {
  local url="$1"
  local retries=80
  local i=0
  while [[ $i -lt $retries ]]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i+1))
    sleep 0.25
  done
  return 1
}

echo "==> API dir: ${API_DIR}"
cd "${API_DIR}"

echo "==> 1) Static checks"
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test ./...

echo "==> 2) Start Redis (ephemeral)"
redis-server --save "" --appendonly no --port "${REDIS_PORT}" >/tmp/auction_redis.log 2>&1 &
REDIS_PID=$!
sleep 0.5

echo "==> 3) Start API server"
CGO_ENABLED=0 AUCTION_GRPC_ADDR=":8081" AUCTION_HTTP_ADDR=":8080" go run ./cmd/server >/tmp/auction_api.log 2>&1 &
SERVER_PID=$!

echo "==> 4) Wait for readiness"
wait_for_http "http://${HTTP_ADDR}/readyz"
echo "Server ready."

echo "==> 5) Integration test (Redis-backed)"
REDIS_ADDR="${REDIS_ADDR}" CGO_ENABLED=0 go test ./internal/engine -run RedisStorePlaceBidIntegration -v

echo "==> 6) Micro benchmark (go test -bench)"
REDIS_ADDR="${REDIS_ADDR}" go test -run '^$' -bench BenchmarkRedisPlaceBid -benchmem ./bench

echo "==> 7) 1M load test (single run)"
CGO_ENABLED=0 go run ./bench/loadtest \
  --grpc_addr="${GRPC_ADDR}" \
  --requests="${REQUESTS}" \
  --concurrency="${CONCURRENCY}" \
  --rpc_timeout_ms="${RPC_TIMEOUT_MS}"

echo "==> 8) Soak runs (${SOAK_RUNS}x)"
for i in $(seq 1 "${SOAK_RUNS}"); do
  echo "--- Soak run ${i}/${SOAK_RUNS} ---"
  CGO_ENABLED=0 go run ./bench/loadtest \
    --grpc_addr="${GRPC_ADDR}" \
    --auction_id="load-auction-soak-${i}" \
    --requests="${REQUESTS}" \
    --concurrency="${CONCURRENCY}" \
    --rpc_timeout_ms="${RPC_TIMEOUT_MS}"
done

echo "==> 9) Metrics snapshot"
curl -fsS "http://${HTTP_ADDR}/metrics" | \
  awk '/auction_bid_requests_total|auction_bid_latency_seconds_bucket|auction_redis_breaker_open|auction_inflight_bids/ {print}'

echo "All benchmark/test stages completed."
