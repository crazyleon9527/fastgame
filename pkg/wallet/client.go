package wallet

import "context"

const (
	TxStatusNotFound    = "not_found"
	TxStatusBetOnly     = "bet_only"
	TxStatusSettled     = "settled"
	TxStatusRolledBack  = "rolled_back"
)

type TxCheckResult struct {
	RoundID   string
	Status    string
	BetAmount float64
	WinAmount float64
}

type Client interface {
	GetBalance(ctx context.Context, merchantID string, userID uint64) (float64, error)
	Bet(ctx context.Context, req BetReq) (*Result, error)
	Win(ctx context.Context, req WinReq) (*Result, error)
	Rollback(ctx context.Context, req RollbackReq) error
	CheckTransaction(ctx context.Context, merchantID string, userID uint64, roundID string) (*TxCheckResult, error)
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
