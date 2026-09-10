package wallet

import "context"

type Client interface {
	GetBalance(ctx context.Context, merchantID string, userID uint64) (float64, error)
	Bet(ctx context.Context, req BetReq) (*Result, error)
	Win(ctx context.Context, req WinReq) (*Result, error)
	Rollback(ctx context.Context, req RollbackReq) error
}

type BetReq struct {
	MerchantID string
	UserID     uint64
	RoundID    string
	Amount     float64
}

type WinReq struct {
	MerchantID string
	UserID     uint64
	RoundID    string
	Amount     float64
}

type RollbackReq struct {
	MerchantID string
	UserID     uint64
	RoundID    string
	Amount     float64
	Reason     string
}

type Result struct {
	Balance float64
}
