package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestDecodeCachedResult(t *testing.T) {
	tests := []struct {
		in   string
		want PlaceBidResult
	}{
		{
			in: "1|accepted|150|u1|3|evt-9|1710000000000",
			want: PlaceBidResult{
				Accepted: true, Reason: "accepted", CurrentPrice: 150,
				WinnerUserID: "u1", Version: 3, EventID: "evt-9", ServerUnixMilli: 1710000000000,
			},
		},
		{
			in:   "0|bid_too_low|100||2||1710000000000",
			want: PlaceBidResult{Reason: "bid_too_low", CurrentPrice: 100, Version: 2, ServerUnixMilli: 1710000000000},
		},
		{
			in:   "0|auction_not_found|0||0||1",
			want: PlaceBidResult{Reason: "auction_not_found", ServerUnixMilli: 1},
		},
		{
			in:   "0|auction_ended",
			want: PlaceBidResult{Reason: "auction_ended"},
		},
		{in: "", want: PlaceBidResult{}},
	}
	for _, tc := range tests {
		got := decodeCachedResult(tc.in)
		if got != tc.want {
			t.Fatalf("decodeCachedResult(%q)\n got %+v\nwant %+v", tc.in, got, tc.want)
		}
	}
}

func TestRedisStoreWithMiniredis(t *testing.T) {
	runEngineStoreTests(t, newMiniredisStore(t))
}

func storeTestConfig() config.Config {
	return config.Config{
		RedisPoolSize:           8,
		BidIdempotencyTTL:       time.Minute,
		StreamReadCount:         32,
		BreakerFailureThreshold: 3,
		BreakerOpenFor:          time.Second,
	}
}

func newMiniredisStore(t *testing.T) *RedisStore {
	t.Helper()
	mr := miniredis.RunT(t)
	return newStoreAt(t, mr.Addr())
}

func newStoreAt(t *testing.T, addr string) *RedisStore {
	t.Helper()
	router, err := NewRedisRouter([]string{addr}, "", 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { router.Close() })
	return NewRedisStore(router, storeTestConfig())
}

func uniqueAuctionID(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s-%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
}

func createOpenAuction(t *testing.T, store *RedisStore, id string, opening int64) {
	t.Helper()
	if err := store.CreateAuction(context.Background(), CreateAuctionRequest{
		AuctionID:         id,
		OpeningPriceCents: opening,
		EndAt:             time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
}

func runEngineStoreTests(t *testing.T, store *RedisStore) {
	ctx := context.Background()

	t.Run("accept and duplicate", func(t *testing.T) {
		id := uniqueAuctionID(t)
		createOpenAuction(t, store, id, 100)
		first, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b1", UserID: "u1", AmountCents: 150})
		if err != nil {
			t.Fatal(err)
		}
		if !first.Accepted || first.Reason != "accepted" || first.CurrentPrice != 150 || first.WinnerUserID != "u1" || first.Version != 1 || first.EventID == "" {
			t.Fatalf("accepted bid: %+v", first)
		}
		dup, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b1", UserID: "u1", AmountCents: 150})
		if err != nil {
			t.Fatal(err)
		}
		if !dup.Duplicate || dup.Version != first.Version || dup.EventID != first.EventID || dup.CurrentPrice != first.CurrentPrice {
			t.Fatalf("duplicate: %+v vs first %+v", dup, first)
		}
	})

	t.Run("bid too low", func(t *testing.T) {
		id := uniqueAuctionID(t)
		createOpenAuction(t, store, id, 100)
		res, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b-low", UserID: "u1", AmountCents: 50})
		if err != nil {
			t.Fatal(err)
		}
		if res.Accepted || res.Reason != "bid_too_low" || res.CurrentPrice != 100 {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("auction not found", func(t *testing.T) {
		id := uniqueAuctionID(t)
		_, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b1", UserID: "u1", AmountCents: 150})
		if err == nil || err.Error() != "auction_not_found" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("not found does not open breaker", func(t *testing.T) {
		id := uniqueAuctionID(t)
		for i := 0; i < 10; i++ {
			_, err := store.PlaceBid(ctx, PlaceBidRequest{
				AuctionID: id, BidID: "b-" + strconv.Itoa(i), UserID: "u1", AmountCents: 150,
			})
			if err == nil || err.Error() != "auction_not_found" {
				t.Fatalf("got %v", err)
			}
		}
		if store.BreakerOpen() {
			t.Fatal("business errors should not open the redis breaker")
		}
	})

	t.Run("ended then closed", func(t *testing.T) {
		id := uniqueAuctionID(t)
		if err := store.CreateAuction(ctx, CreateAuctionRequest{
			AuctionID:         id,
			OpeningPriceCents: 100,
			EndAt:             time.Now().Add(-time.Second),
		}); err != nil {
			t.Fatal(err)
		}
		ended, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b-end", UserID: "u1", AmountCents: 200})
		if err != nil {
			t.Fatal(err)
		}
		if ended.Accepted || ended.Reason != "auction_ended" {
			t.Fatalf("ended: %+v", ended)
		}
		closed, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b-closed", UserID: "u2", AmountCents: 300})
		if err != nil {
			t.Fatal(err)
		}
		if closed.Accepted || closed.Reason != "auction_closed" {
			t.Fatalf("closed: %+v", closed)
		}
	})

	t.Run("read events", func(t *testing.T) {
		id := uniqueAuctionID(t)
		createOpenAuction(t, store, id, 100)
		if _, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b1", UserID: "u1", AmountCents: 150}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "b2", UserID: "u2", AmountCents: 175}); err != nil {
			t.Fatal(err)
		}
		events, err := store.ReadEvents(ctx, map[string]string{id: "0-0"}, 32)
		if err != nil && err != redis.Nil {
			t.Fatal(err)
		}
		if len(events) != 2 {
			t.Fatalf("got %d events: %+v", len(events), events)
		}
		if events[0].BidID != "b1" || events[0].AmountCents != 150 || events[0].AuctionID != id || events[0].ID == "" {
			t.Fatalf("event 0: %+v", events[0])
		}
		if events[1].BidID != "b2" || events[1].AmountCents != 175 || events[1].UserID != "u2" {
			t.Fatalf("event 1: %+v", events[1])
		}
		again, err := store.ReadEvents(ctx, map[string]string{id: events[1].ID}, 32)
		if err != nil && err != redis.Nil {
			t.Fatal(err)
		}
		if len(again) != 0 {
			t.Fatalf("expected no new events, got %+v", again)
		}
	})

	t.Run("max bid wins under concurrency", func(t *testing.T) {
		id := uniqueAuctionID(t)
		createOpenAuction(t, store, id, 1)
		const n = 40
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			amount := int64(2 + i)
			go func(i int, amount int64) {
				defer wg.Done()
				_, _ = store.PlaceBid(ctx, PlaceBidRequest{
					AuctionID: id, BidID: "b-" + strconv.Itoa(i), UserID: "u-" + strconv.Itoa(i), AmountCents: amount,
				})
			}(i, amount)
		}
		wg.Wait()
		res, err := store.PlaceBid(ctx, PlaceBidRequest{AuctionID: id, BidID: "probe", UserID: "probe", AmountCents: 1})
		if err != nil {
			t.Fatal(err)
		}
		want := int64(2 + n - 1)
		if res.CurrentPrice != want {
			t.Fatalf("current price %d, want max bid %d (reason=%s)", res.CurrentPrice, want, res.Reason)
		}
	})
}
