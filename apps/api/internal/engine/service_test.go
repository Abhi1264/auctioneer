package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingStore struct {
	created CreateAuctionRequest
	bids    []PlaceBidRequest
}

func (r *recordingStore) CreateAuction(_ context.Context, req CreateAuctionRequest) error {
	r.created = req
	return nil
}

func (r *recordingStore) PlaceBid(_ context.Context, req PlaceBidRequest) (PlaceBidResult, error) {
	r.bids = append(r.bids, req)
	return PlaceBidResult{Accepted: true, Reason: "accepted"}, nil
}

func (r *recordingStore) ReadEvents(context.Context, map[string]string, int64) ([]AuctionEvent, error) {
	return nil, nil
}

func TestServicePlaceBidValidation(t *testing.T) {
	store := &recordingStore{}
	svc := NewService(store, time.Minute)

	tests := []struct {
		name string
		req  PlaceBidRequest
		want error
	}{
		{name: "missing auction id", req: PlaceBidRequest{BidID: "b", UserID: "u", AmountCents: 1}, want: ErrInvalidAuctionID},
		{name: "missing bid id", req: PlaceBidRequest{AuctionID: "a", UserID: "u", AmountCents: 1}, want: ErrInvalidBidID},
		{name: "missing user id", req: PlaceBidRequest{AuctionID: "a", BidID: "b", AmountCents: 1}, want: ErrInvalidUserID},
		{name: "zero amount", req: PlaceBidRequest{AuctionID: "a", BidID: "b", UserID: "u", AmountCents: 0}, want: ErrInvalidAmount},
		{name: "negative amount", req: PlaceBidRequest{AuctionID: "a", BidID: "b", UserID: "u", AmountCents: -1}, want: ErrInvalidAmount},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.PlaceBid(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if len(store.bids) != 0 {
				t.Fatal("invalid request should not reach the store")
			}
		})
	}
}

func TestServicePlaceBidForwards(t *testing.T) {
	store := &recordingStore{}
	svc := NewService(store, time.Minute)
	req := PlaceBidRequest{AuctionID: "a1", BidID: "b1", UserID: "u1", AmountCents: 1000}
	res, err := svc.PlaceBid(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || len(store.bids) != 1 || store.bids[0] != req {
		t.Fatalf("forward mismatch: res=%+v store=%+v", res, store.bids)
	}
}

func TestServiceCreateAuctionValidation(t *testing.T) {
	store := &recordingStore{}
	svc := NewService(store, time.Minute)
	if err := svc.CreateAuction(context.Background(), CreateAuctionRequest{}); !errors.Is(err, ErrInvalidAuctionID) {
		t.Fatalf("got %v, want %v", err, ErrInvalidAuctionID)
	}
}

func TestServiceCreateAuctionDefaultDuration(t *testing.T) {
	store := &recordingStore{}
	svc := NewService(store, 5*time.Minute)
	before := time.Now().UTC()
	if err := svc.CreateAuction(context.Background(), CreateAuctionRequest{AuctionID: "a1", OpeningPriceCents: 10}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()
	got := store.created.EndAt
	if got.Before(before.Add(5*time.Minute-time.Second)) || got.After(after.Add(5*time.Minute+time.Second)) {
		t.Fatalf("EndAt %s not around now+5m", got)
	}
}

func TestServiceCreateAuctionKeepsEndAt(t *testing.T) {
	store := &recordingStore{}
	svc := NewService(store, time.Hour)
	end := time.Now().UTC().Add(2 * time.Hour)
	if err := svc.CreateAuction(context.Background(), CreateAuctionRequest{AuctionID: "a1", EndAt: end}); err != nil {
		t.Fatal(err)
	}
	if !store.created.EndAt.Equal(end) {
		t.Fatalf("EndAt mutated: got %s want %s", store.created.EndAt, end)
	}
}
