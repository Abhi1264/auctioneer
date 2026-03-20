package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/obs"
	wsapi "github.com/Abhi1264/auctioneer/apps/api/internal/transport/ws"
)

type EventDispatcher struct {
	store   Store
	hub     *wsapi.Hub
	logger  *slog.Logger
	metrics *obs.Metrics
	offsets map[string]string
	subs    map[string]context.CancelFunc
}

func NewEventDispatcher(store Store, hub *wsapi.Hub, logger *slog.Logger, metrics *obs.Metrics) *EventDispatcher {
	return &EventDispatcher{
		store:   store,
		hub:     hub,
		logger:  logger,
		metrics: metrics,
		offsets: make(map[string]string),
		subs:    make(map[string]context.CancelFunc),
	}
}

func (d *EventDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			for _, cancel := range d.subs {
				cancel()
			}
			return
		case <-ticker.C:
			for auctionID := range d.hub.KnownAuctions() {
				if _, ok := d.offsets[auctionID]; !ok {
					d.offsets[auctionID] = "0-0"
				}
				if _, ok := d.subs[auctionID]; !ok {
					if redisStore, ok := d.store.(*RedisStore); ok {
						subCtx, cancel := context.WithCancel(ctx)
						d.subs[auctionID] = cancel
						go func(id string) {
							err := redisStore.SubscribeAuction(subCtx, id, func(event AuctionEvent) {
								d.hub.Broadcast(id, wsapi.Message{
									AuctionID: event.AuctionID,
									Type:      "bid_accepted",
									BidID:     event.BidID,
									UserID:    event.UserID,
									Amount:    event.AmountCents,
									Version:   event.Version,
									Timestamp: event.ServerUnixMilli,
								})
							})
							if err != nil {
								d.logger.Warn("pubsub subscriber ended with error", "auction_id", id, "err", err)
							}
						}(auctionID)
					}
				}
			}
			if len(d.offsets) == 0 {
				continue
			}
			events, err := d.store.ReadEvents(ctx, d.offsets, 128)
			if err != nil {
				d.logger.Warn("failed to read events", "err", err)
				continue
			}
			for _, event := range events {
				d.offsets[event.AuctionID] = event.ID
				if d.metrics != nil {
					d.metrics.StreamDispatchLag.Set(float64(time.Now().UnixMilli() - event.ServerUnixMilli))
				}
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
