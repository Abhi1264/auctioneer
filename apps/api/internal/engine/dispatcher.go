package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/obs"
	wsapi "github.com/Abhi1264/auctioneer/apps/api/internal/transport/ws"
)

type EventDispatcher struct {
	store       Store
	hub         *wsapi.Hub
	logger      *slog.Logger
	metrics     *obs.Metrics
	streamCount int64
	offsets     map[string]string
}

func NewEventDispatcher(store Store, hub *wsapi.Hub, logger *slog.Logger, metrics *obs.Metrics, streamCount int64) *EventDispatcher {
	if streamCount <= 0 {
		streamCount = 256
	}
	return &EventDispatcher{
		store:       store,
		hub:         hub,
		logger:      logger,
		metrics:     metrics,
		streamCount: streamCount,
		offsets:     make(map[string]string),
	}
}

func (d *EventDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			known := d.hub.KnownAuctions()
			for id := range d.offsets {
				if _, ok := known[id]; !ok {
					delete(d.offsets, id)
				}
			}
			for auctionID := range known {
				if _, ok := d.offsets[auctionID]; !ok {
					d.offsets[auctionID] = "0-0"
				}
			}
			if len(d.offsets) == 0 {
				continue
			}
			events, err := d.store.ReadEvents(ctx, d.offsets, d.streamCount)
			if err != nil {
				d.logger.Warn("failed to read events", "err", err)
				continue
			}
			if len(events) > 0 {
				d.metrics.StreamDispatchLag.Set(float64(time.Now().UnixMilli() - events[0].ServerUnixMilli))
			}
			for _, event := range events {
				d.offsets[event.AuctionID] = event.ID
				d.hub.Broadcast(event.AuctionID, wsapi.Message{
					AuctionID: event.AuctionID,
					Type:      "bid_accepted",
					BidID:     event.BidID,
					UserID:    event.UserID,
					Amount:    event.AmountCents,
					Version:   event.Version,
					Timestamp: event.ServerUnixMilli,
				})
			}
		}
	}
}
