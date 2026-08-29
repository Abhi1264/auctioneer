package engine

import (
	"testing"

	"github.com/Abhi1264/auctioneer/apps/api/internal/config"
)

func TestRedisStorePlaceBidIntegration(t *testing.T) {
	addr := config.RedisAddrFromEnv()
	if addr == "" {
		t.Skip("set REDIS_ADDR or REDIS_ADDRS for integration test")
	}
	runEngineStoreTests(t, newStoreAt(t, addr))
}
