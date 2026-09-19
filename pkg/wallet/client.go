package wallet

import (
	"context"
	"errors"

	"fastgame/pkg/money"
)

var (
	ErrCircuitOpen      = errors.New("wallet circuit open")
	ErrSlowResponse     = errors.New("wallet slow response")
	ErrInsufficientFund = errors.New("insufficient balance")
	ErrMerchantDisabled = errors.New("merchant wallet disabled")
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
	// GetBalance 查询余额
	GetBalance(ctx context.Context, merchantID, userID, currency string) (money.Amount, error)

	// Settle 免转核心原子结算接口 (Bet + Win 单次完成)
	Settle(ctx context.Context, req SettleReq) (*SettleResult, error)

	// Bet 与 Win 保留兼容旧链路或单边调用
	Bet(ctx context.Context, req BetReq) (*Result, error)
	Win(ctx context.Context, req WinReq) (*Result, error)

	// Rollback 冲正/回滚补偿接口
	Rollback(ctx context.Context, req RollbackReq) error

	// CheckTransaction 查单对账
	CheckTransaction(ctx context.Context, merchantID, userID, roundID string) (*TxCheckResult, error)

	// TransferIn/Out 针对转账钱包
	TransferIn(ctx context.Context, req TransferReq) (*TransferResult, error)
	TransferOut(ctx context.Context, req TransferReq) (*TransferResult, error)
}

type SettleReq struct {
	MerchantID    string
	UserID        string
	GameCode      string
	RoundID       string
	TransactionID string
	Currency      string
	BetAmount     money.Amount
	WinAmount     money.Amount
}

type SettleResult struct {
	Balance       money.Amount
	TransactionID string
}

type BetReq struct {
	MerchantID string
	UserID     string
	RoundID    string
	Amount     money.Amount
}

type WinReq struct {
	MerchantID string
	UserID     string
	RoundID    string
	Amount     money.Amount
}

type RollbackReq struct {
	MerchantID            string
	UserID                string
	RoundID               string
	RollbackTransactionID string
	NewTransactionID      string
	Amount                money.Amount
	Reason                string
}

type Result struct {
	Balance money.Amount
}

type TransferReq struct {
	MerchantID string
	UserID     string
	TransferID string
	Amount     money.Amount
	Currency   string
}

type TransferResult struct {
	GameBalance money.Amount
	TransferID  string
}
