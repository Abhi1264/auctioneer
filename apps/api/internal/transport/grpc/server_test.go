package grpc

import (
	"context"
	"testing"
	"time"

	auctionv1 "github.com/Abhi1264/auctioneer/apps/api/auction/v1"
	eng "github.com/Abhi1264/auctioneer/apps/api/internal/engine"
	"github.com/Abhi1264/auctioneer/apps/api/internal/obs"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubStore struct {
	placeRes eng.PlaceBidResult
	placeErr error
}

func (s *stubStore) CreateAuction(context.Context, eng.CreateAuctionRequest) error { return nil }
func (s *stubStore) PlaceBid(context.Context, eng.PlaceBidRequest) (eng.PlaceBidResult, error) {
	return s.placeRes, s.placeErr
}
func (s *stubStore) ReadEvents(context.Context, map[string]string, int64) ([]eng.AuctionEvent, error) {
	return nil, nil
}

func newTestAPI(t *testing.T, store eng.Store, maxInFlight int, breakerOpen func() bool) *API {
	t.Helper()
	if breakerOpen == nil {
		breakerOpen = func() bool { return false }
	}
	return &API{
		svc:         eng.NewService(store, time.Minute),
		inflightSem: make(chan struct{}, maxInFlight),
		metrics:     obs.NewMetrics(prometheus.NewRegistry()),
		breakerOpen: breakerOpen,
	}
}

func TestPlaceBidInvalidArgument(t *testing.T) {
	api := newTestAPI(t, &stubStore{}, 8, nil)
	_, err := api.PlaceBid(context.Background(), &auctionv1.PlaceBidRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}

func TestPlaceBidBreakerUnavailable(t *testing.T) {
	api := newTestAPI(t, &stubStore{placeErr: eng.ErrRedisBreakerOpen}, 8, func() bool { return true })
	_, err := api.PlaceBid(context.Background(), &auctionv1.PlaceBidRequest{
		AuctionId: "a", BidId: "b", UserId: "u", AmountCents: 1,
	})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("got %v", err)
	}
}

func TestPlaceBidOverloaded(t *testing.T) {
	api := newTestAPI(t, &stubStore{placeRes: eng.PlaceBidResult{Accepted: true}}, 1, nil)
	api.inflightSem <- struct{}{}
	_, err := api.PlaceBid(context.Background(), &auctionv1.PlaceBidRequest{
		AuctionId: "a", BidId: "b", UserId: "u", AmountCents: 1,
	})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("got %v", err)
	}
}

func TestPlaceBidAccepted(t *testing.T) {
	api := newTestAPI(t, &stubStore{placeRes: eng.PlaceBidResult{
		Accepted: true, Reason: "accepted", CurrentPrice: 150, WinnerUserID: "u1", Version: 2, EventID: "e1",
	}}, 8, nil)
	res, err := api.PlaceBid(context.Background(), &auctionv1.PlaceBidRequest{
		AuctionId: "a", BidId: "b", UserId: "u1", AmountCents: 150,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || res.CurrentPriceCents != 150 || res.WinnerUserId != "u1" || res.EventId != "e1" {
		t.Fatalf("got %+v", res)
	}
}

func TestCreateAuctionMissingID(t *testing.T) {
	api := newTestAPI(t, &stubStore{}, 8, nil)
	_, err := api.CreateAuction(context.Background(), &auctionv1.CreateAuctionRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v", err)
	}
}
