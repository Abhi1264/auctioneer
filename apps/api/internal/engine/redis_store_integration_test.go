package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/config"
)

func TestRedisStorePlaceBidIntegration(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("set REDIS_ADDR for integration test")
	}

	cfg := config.Config{
		RedisAddresses:          []string{addr},
		RedisPoolSize:           32,
		BidIdempotencyTTL:       time.Minute,
		StreamReadCount:         32,
		BreakerFailureThreshold: 5,
		BreakerOpenFor:          100 * time.Millisecond,
	}

	router, err := NewRedisRouter(cfg.RedisAddresses, "", 0, cfg.RedisPoolSize)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	defer router.Close()
	store := NewRedisStore(router, cfg)

	ctx := context.Background()
	auctionID := "it-auction"
	err = store.CreateAuction(ctx, CreateAuctionRequest{
		AuctionID:         auctionID,
		OpeningPriceCents: 100,
		EndAt:             time.Now().Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create auction: %v", err)
	}

	first, err := store.PlaceBid(ctx, PlaceBidRequest{
		AuctionID:   auctionID,
		BidID:       "it-bid-1",
		UserID:      "u1",
		AmountCents: 150,
	})
	if err != nil {
		t.Fatalf("place bid: %v", err)
	}
	if !first.Accepted {
		t.Fatalf("expected accepted result, got %+v", first)
	}

	dup, err := store.PlaceBid(ctx, PlaceBidRequest{
		AuctionID:   auctionID,
		BidID:       "it-bid-1",
		UserID:      "u1",
		AmountCents: 150,
	})
	if err != nil {
		t.Fatalf("duplicate place bid: %v", err)
	}
	if !dup.Duplicate {
		t.Fatalf("expected duplicate, got %+v", dup)
	}
}
