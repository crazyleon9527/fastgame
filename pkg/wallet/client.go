package wallet

import (
	"context"
	"errors"

	"fastgame/pkg/money"
)

var (
	ErrCircuitOpen  = errors.New("wallet circuit open")
	ErrSlowResponse = errors.New("wallet slow response")
)

const (
	TxStatusNotFound   = "not_found"
	TxStatusBetOnly    = "bet_only"
	TxStatusSettled    = "settled"
	TxStatusRolledBack = "rolled_back"
)

type TxCheckResult struct {
	RoundID   string
	Status    string
	BetAmount money.Amount
	WinAmount money.Amount
}

type Client interface {
	GetBalance(ctx context.Context, merchantID string, userID uint64) (money.Amount, error)
	Bet(ctx context.Context, req BetReq) (*Result, error)
	Win(ctx context.Context, req WinReq) (*Result, error)
	Rollback(ctx context.Context, req RollbackReq) error
	CheckTransaction(ctx context.Context, merchantID string, userID uint64, roundID string) (*TxCheckResult, error)
}

type BetReq struct {
	MerchantID string
	UserID     uint64
	RoundID    string
	Amount     money.Amount
}

type WinReq struct {
	MerchantID string
	UserID     uint64
	RoundID    string
	Amount     money.Amount
}

type RollbackReq struct {
	MerchantID string
	UserID     uint64
	RoundID    string
	Amount     money.Amount
	Reason     string
}

type Result struct {
	Balance money.Amount
}
