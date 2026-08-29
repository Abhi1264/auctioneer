package engine

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/obs"
	wsapi "github.com/Abhi1264/auctioneer/apps/api/internal/transport/ws"
	"github.com/prometheus/client_golang/prometheus"
)

type scriptedStore struct {
	events []AuctionEvent
}

func (s *scriptedStore) CreateAuction(context.Context, CreateAuctionRequest) error { return nil }
func (s *scriptedStore) PlaceBid(context.Context, PlaceBidRequest) (PlaceBidResult, error) {
	return PlaceBidResult{}, nil
}
func (s *scriptedStore) ReadEvents(context.Context, map[string]string, int64) ([]AuctionEvent, error) {
	ev := s.events
	s.events = nil
	return ev, nil
}

func waitForAuction(t *testing.T, hub *wsapi.Hub, auctionID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := hub.KnownAuctions()[auctionID]; ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("auction %s never registered", auctionID)
}

func TestDispatcherBroadcastsStreamEvents(t *testing.T) {
	hub := wsapi.NewHub(8, prometheus.NewCounter(prometheus.CounterOpts{Name: "test_ws_dropped"}))
	go hub.Run()
	client := wsapi.NewBufferedClient(8)
	hub.Register("auc-1", client)
	waitForAuction(t, hub, "auc-1")

	store := &scriptedStore{events: []AuctionEvent{{
		ID: "1-0", AuctionID: "auc-1", BidID: "b1", UserID: "u1",
		AmountCents: 150, Version: 1, ServerUnixMilli: time.Now().UnixMilli(),
	}}}
	metrics := obs.NewMetrics(prometheus.NewRegistry())
	d := NewEventDispatcher(store, hub, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Run(ctx)

	select {
	case payload := <-client.Messages():
		var msg wsapi.Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type != "bid_accepted" || msg.BidID != "b1" || msg.Amount != 150 || msg.AuctionID != "auc-1" {
			t.Fatalf("got %+v", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for websocket fanout")
	}
}
