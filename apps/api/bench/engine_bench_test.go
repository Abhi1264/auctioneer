package bench

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/config"
	"github.com/Abhi1264/auctioneer/apps/api/internal/engine"
)

func BenchmarkRedisPlaceBid(b *testing.B) {
	addr := config.RedisAddrFromEnv()
	if addr == "" {
		b.Skip("set REDIS_ADDR or REDIS_ADDRS for benchmark")
	}
	cfg := config.Config{
		RedisAddresses:          []string{addr},
		RedisPoolSize:           128,
		BidIdempotencyTTL:       time.Minute,
		StreamReadCount:         128,
		BreakerFailureThreshold: 20,
		BreakerOpenFor:          200 * time.Millisecond,
	}
	router, err := engine.NewRedisRouter(cfg.RedisAddresses, "", 0, cfg.RedisPoolSize)
	if err != nil {
		b.Fatalf("router: %v", err)
	}
	defer router.Close()
	store := engine.NewRedisStore(router, cfg)
	svc := engine.NewService(store, 0)

	ctx := context.Background()
	auctionID := "bench-auction"
	if err := svc.CreateAuction(ctx, engine.CreateAuctionRequest{
		AuctionID:         auctionID,
		OpeningPriceCents: 1,
		EndAt:             time.Now().Add(time.Hour),
	}); err != nil {
		b.Fatalf("create auction: %v", err)
	}

	b.ReportAllocs()
	
	for i := 0; b.Loop(); i++ {
		_, err := svc.PlaceBid(ctx, engine.PlaceBidRequest{
			AuctionID:   auctionID,
			BidID:       "bench-bid-" + strconv.Itoa(i),
			UserID:      "bench-user",
			AmountCents: int64(i + 2),
		})
		if err != nil {
			b.Fatalf("place bid: %v", err)
		}
	}
}
