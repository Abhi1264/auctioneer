package ws

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestHubRegisterBroadcastUnregister(t *testing.T) {
	hub := NewHub(4, prometheus.NewCounter(prometheus.CounterOpts{Name: "test_hub_dropped"}))
	go hub.Run()

	c := NewBufferedClient(4)
	hub.Register("a1", c)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := hub.KnownAuctions()["a1"]; ok {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := hub.KnownAuctions()["a1"]; !ok {
		t.Fatal("auction not registered")
	}

	hub.Broadcast("a1", Message{Type: "bid_accepted", AuctionID: "a1", BidID: "b1", Amount: 9})
	select {
	case payload := <-c.Messages():
		if len(payload) == 0 {
			t.Fatal("empty payload")
		}
	case <-time.After(time.Second):
		t.Fatal("no broadcast")
	}

	hub.Unregister("a1", c)
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := hub.KnownAuctions()["a1"]; !ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("auction still in hub after unregister")
}
