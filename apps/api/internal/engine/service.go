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
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateAuction(ctx context.Context, req CreateAuctionRequest) error {
	if req.AuctionID == "" {
		return ErrInvalidAuctionID
	}
	if req.EndAt.IsZero() {
		req.EndAt = time.Now().UTC().Add(10 * time.Minute)
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

func (s *Service) ReadEvents(ctx context.Context, offsets map[string]string, count int64) ([]AuctionEvent, error) {
	return s.store.ReadEvents(ctx, offsets, count)
}
