package engine

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidAuctionID = errors.New("auction_id is required")
	ErrInvalidBidID     = errors.New("bid_id is required")
	ErrInvalidUserID    = errors.New("user_id is required")
	ErrInvalidAmount    = errors.New("amount must be > 0")
)

type Store interface {
	CreateAuction(ctx context.Context, req CreateAuctionRequest) error
	PlaceBid(ctx context.Context, req PlaceBidRequest) (PlaceBidResult, error)
	ReadEvents(ctx context.Context, offsetByAuction map[string]string, count int64) ([]AuctionEvent, error)
}

type Service struct {
	store           Store
	defaultDuration time.Duration
}

func NewService(store Store, defaultDuration time.Duration) *Service {
	if defaultDuration <= 0 {
		defaultDuration = 10 * time.Minute
	}
	return &Service{store: store, defaultDuration: defaultDuration}
}

func (s *Service) CreateAuction(ctx context.Context, req CreateAuctionRequest) error {
	if req.AuctionID == "" {
		return ErrInvalidAuctionID
	}
	if req.EndAt.IsZero() {
		req.EndAt = time.Now().UTC().Add(s.defaultDuration)
	}
	return s.store.CreateAuction(ctx, req)
}

func (s *Service) PlaceBid(ctx context.Context, req PlaceBidRequest) (PlaceBidResult, error) {
	if req.AuctionID == "" {
		return PlaceBidResult{}, ErrInvalidAuctionID
	}
	if req.BidID == "" {
		return PlaceBidResult{}, ErrInvalidBidID
	}
	if req.UserID == "" {
		return PlaceBidResult{}, ErrInvalidUserID
	}
	if req.AmountCents <= 0 {
		return PlaceBidResult{}, ErrInvalidAmount
	}
	return s.store.PlaceBid(ctx, req)
}
