package engine

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu      sync.Mutex
	price   int64
	version int64
	winner  string
	seen    map[string]PlaceBidResult
}

func (m *memoryStore) CreateAuction(context.Context, CreateAuctionRequest) error { return nil }
func (m *memoryStore) ReadEvents(context.Context, map[string]string, int64) ([]AuctionEvent, error) {
	return nil, nil
}

func (m *memoryStore) PlaceBid(_ context.Context, req PlaceBidRequest) (PlaceBidResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cached, ok := m.seen[req.BidID]; ok {
		cached.Duplicate = true
		return cached, nil
	}
	if req.AmountCents <= m.price {
		res := PlaceBidResult{
			Accepted:        false,
			Reason:          "bid_too_low",
			CurrentPrice:    m.price,
			WinnerUserID:    m.winner,
			Version:         m.version,
			ServerUnixMilli: time.Now().UnixMilli(),
		}
		m.seen[req.BidID] = res
		return res, nil
	}
	m.version++
	m.price = req.AmountCents
	m.winner = req.UserID
	res := PlaceBidResult{
		Accepted:        true,
		Reason:          "accepted",
		CurrentPrice:    m.price,
		WinnerUserID:    m.winner,
		Version:         m.version,
		ServerUnixMilli: time.Now().UnixMilli(),
	}
	m.seen[req.BidID] = res
	return res, nil
}

func TestServiceValidation(t *testing.T) {
	svc := NewService(&memoryStore{seen: map[string]PlaceBidResult{}}, 0)
	_, err := svc.PlaceBid(context.Background(), PlaceBidRequest{})
	if err == nil {
		t.Fatal("expected error for empty request")
	}
}

func TestServiceIdempotency(t *testing.T) {
	svc := NewService(&memoryStore{seen: map[string]PlaceBidResult{}}, 0)
	req := PlaceBidRequest{
		AuctionID:   "a1",
		BidID:       "b1",
		UserID:      "u1",
		AmountCents: 1000,
	}
	first, err := svc.PlaceBid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	second, err := svc.PlaceBid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !first.Accepted || !second.Duplicate {
		t.Fatalf("expected accepted then duplicate, got %+v and %+v", first, second)
	}
}

func TestMonotonicWinnerInvariantUnderConcurrency(t *testing.T) {
	store := &memoryStore{seen: map[string]PlaceBidResult{}}
	svc := NewService(store, 0)

	const workers = 200
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		amount := int64(1000 + i)
		go func(idx int, bidAmount int64) {
			defer wg.Done()
			_, _ = svc.PlaceBid(context.Background(), PlaceBidRequest{
				AuctionID:   "a1",
				BidID:       "b-" + strconv.Itoa(idx),
				UserID:      "u",
				AmountCents: bidAmount,
			})
		}(i, amount)
	}
	wg.Wait()

	if store.version <= 0 {
		t.Fatalf("expected version > 0, got %d", store.version)
	}
	if store.price < 1000 {
		t.Fatalf("expected price >= 1000, got %d", store.price)
	}
}
