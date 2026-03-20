package obs

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	BidRequestsTotal  *prometheus.CounterVec
	BidLatencySeconds prometheus.Histogram
	InFlightBids      prometheus.Gauge
	RedisBreakerOpen  prometheus.Gauge
	WsDroppedClients  prometheus.Counter
	StreamDispatchLag prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		BidRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auction_bid_requests_total",
			Help: "Total bid requests by outcome.",
		}, []string{"outcome"}),
		BidLatencySeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auction_bid_latency_seconds",
			Help:    "Bid request latency.",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12),
		}),
		InFlightBids: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auction_inflight_bids",
			Help: "Current number of in-flight bid requests.",
		}),
		RedisBreakerOpen: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auction_redis_breaker_open",
			Help: "Redis breaker state (1=open, 0=closed).",
		}),
		WsDroppedClients: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auction_ws_dropped_clients_total",
			Help: "Total websocket clients dropped due to backpressure.",
		}),
		StreamDispatchLag: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auction_stream_dispatch_lag_ms",
			Help: "Observed stream dispatch lag in milliseconds.",
		}),
	}
	reg.MustRegister(
		m.BidRequestsTotal,
		m.BidLatencySeconds,
		m.InFlightBids,
		m.RedisBreakerOpen,
		m.WsDroppedClients,
		m.StreamDispatchLag,
	)
	return m
}

func ObserveLatency(h prometheus.Histogram, start time.Time) {
	h.Observe(time.Since(start).Seconds())
}
