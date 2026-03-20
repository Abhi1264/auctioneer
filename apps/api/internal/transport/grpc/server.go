package grpc

import (
	"context"
	"errors"
	"net"
	"time"

	auctionv1 "github.com/Abhi1264/auctioneer/apps/api/auction/v1"
	eng "github.com/Abhi1264/auctioneer/apps/api/internal/engine"
	"github.com/Abhi1264/auctioneer/apps/api/internal/obs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type API struct {
	auctionv1.UnimplementedAuctionServiceServer
	svc         *eng.Service
	inflightSem chan struct{}
	metrics     *obs.Metrics
	breakerOpen func() bool
}

func NewServer(addr string, svc *eng.Service, maxInFlight int, metrics *obs.Metrics, breakerOpen func() bool) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	api := &API{
		svc:         svc,
		inflightSem: make(chan struct{}, maxInFlight),
		metrics:     metrics,
		breakerOpen: breakerOpen,
	}

	server := grpc.NewServer(
		grpc.MaxConcurrentStreams(uint32(maxInFlight)),
	)
	auctionv1.RegisterAuctionServiceServer(server, api)

	return server, lis, nil
}

func (a *API) PlaceBid(ctx context.Context, req *auctionv1.PlaceBidRequest) (*auctionv1.PlaceBidResponse, error) {
	start := time.Now()
	if a.metrics != nil {
		a.metrics.InFlightBids.Inc()
		defer a.metrics.InFlightBids.Dec()
		defer obs.ObserveLatency(a.metrics.BidLatencySeconds, start)
	}
	select {
	case a.inflightSem <- struct{}{}:
		defer func() { <-a.inflightSem }()
	default:
		if a.metrics != nil {
			a.metrics.BidRequestsTotal.WithLabelValues("overloaded").Inc()
		}
		return nil, status.Error(codes.ResourceExhausted, "server overloaded")
	}

	ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()

	res, err := a.svc.PlaceBid(ctx, eng.PlaceBidRequest{
		AuctionID:       req.GetAuctionId(),
		BidID:           req.GetBidId(),
		UserID:          req.GetUserId(),
		AmountCents:     req.GetAmountCents(),
		ClientUnixMilli: req.GetClientTsMs(),
	})
	if err != nil {
		if a.metrics != nil {
			a.metrics.BidRequestsTotal.WithLabelValues("error").Inc()
		}
		if a.breakerOpen != nil && a.breakerOpen() {
			a.metrics.RedisBreakerOpen.Set(1)
		} else if a.metrics != nil {
			a.metrics.RedisBreakerOpen.Set(0)
		}
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, err.Error())
		case errors.Is(err, eng.ErrRedisBreakerOpen):
			return nil, status.Error(codes.Unavailable, err.Error())
		case errors.Is(err, eng.ErrInvalidAuctionID), errors.Is(err, eng.ErrInvalidBidID), errors.Is(err, eng.ErrInvalidUserID), errors.Is(err, eng.ErrInvalidAmount):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if a.metrics != nil {
		if res.Accepted {
			a.metrics.BidRequestsTotal.WithLabelValues("accepted").Inc()
		} else {
			a.metrics.BidRequestsTotal.WithLabelValues("rejected").Inc()
		}
	}
	return &auctionv1.PlaceBidResponse{
		Accepted:          res.Accepted,
		Duplicate:         res.Duplicate,
		Reason:            res.Reason,
		CurrentPriceCents: res.CurrentPrice,
		WinnerUserId:      res.WinnerUserID,
		Version:           res.Version,
		EventId:           res.EventID,
		ServerTsMs:        res.ServerUnixMilli,
	}, nil
}

func (a *API) CreateAuction(ctx context.Context, req *auctionv1.CreateAuctionRequest) (*auctionv1.Empty, error) {
	duration := time.Duration(req.GetDurationSec()) * time.Second
	if duration <= 0 {
		duration = 10 * time.Minute
	}
	if err := a.svc.CreateAuction(ctx, eng.CreateAuctionRequest{
		AuctionID:         req.GetAuctionId(),
		OpeningPriceCents: req.GetOpeningPriceCents(),
		EndAt:             time.Now().UTC().Add(duration),
	}); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &auctionv1.Empty{}, nil
}
