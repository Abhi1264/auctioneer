package engine

import "time"

type PlaceBidRequest struct {
	AuctionID       string
	BidID           string
	UserID          string
	AmountCents     int64
	ClientUnixMilli int64
}

type PlaceBidResult struct {
	Accepted        bool
	Duplicate       bool
	Reason          string
	CurrentPrice    int64
	WinnerUserID    string
	Version         int64
	EventID         string
	ServerUnixMilli int64
}

type CreateAuctionRequest struct {
	AuctionID         string
	OpeningPriceCents int64
	EndAt             time.Time
}

type AuctionState struct {
	AuctionID      string
	Status         string
	CurrentPrice   int64
	WinnerUserID   string
	Version        int64
	EndAtUnixMilli int64
}

type AuctionEvent struct {
	ID              string
	AuctionID       string
	BidID           string
	UserID          string
	AmountCents     int64
	Version         int64
	ServerUnixMilli int64
}
