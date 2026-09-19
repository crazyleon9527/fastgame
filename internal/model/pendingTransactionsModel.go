package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	PendingTxPhaseBetDebited = "bet_debited"
	PendingTxPhaseWinPending = "win_pending"
	PendingTxPhaseSettled    = "settled"
	PendingTxPhaseOrphan     = "orphan"

	PendingTxActionSettleWin   = "settle_win"
	PendingTxActionRollbackBet = "rollback_bet"
	PendingTxActionNone        = "none"

	PendingTxStatusPending = "pending"
	PendingTxStatusDone    = "done"
	PendingTxStatusFailed  = "failed"
)

// PendingTransaction 对应表 pending_transactions：孤儿注单自动对账补偿
type PendingTransaction struct {
	Id              uint64         `db:"id"`                // 主键
	TraceID         string         `db:"trace_id"`          // 全链路 TraceID
	RoundID         string         `db:"round_id"`          // 局 ID，同时作为 nonce
	MerchantID      uint64         `db:"merchant_id"`       // 商户 ID（merchants.id），唯一键前导列
	MerchantCode    string         `db:"merchant_code"`     // 商户编码
	UserID          string         `db:"user_id"`           // 玩家 ID
	GameCode        string         `db:"game_code"`         // 游戏编码
	Phase           string         `db:"phase"`             // 阶段：bet_debited/win_pending/settled/orphan
	Status          string         `db:"status"`            // 状态：pending/done/failed
	BetAmount       int64          `db:"bet_amount"`        // 下注额，minor units（scale=10000）
	WinAmount       int64          `db:"win_amount"`        // 派彩额，minor units
	ExpectedAction  string         `db:"expected_action"`   // 期望补偿动作：settle_win/rollback_bet/none
	WalletBetStatus string         `db:"wallet_bet_status"` // 钱包扣款状态
	WalletWinStatus string         `db:"wallet_win_status"` // 钱包派彩状态
	RetryCount      int64          `db:"retry_count"`       // 已重试次数
	LastError       sql.NullString `db:"last_error"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

type PendingTransactionsModel interface {
	Insert(ctx context.Context, data *PendingTransaction) error
	// MarkSettled / MarkWinPending / FindByRoundID 都按 (merchant_id, round_id) 定位行。
	// round_id 只在商户内唯一（uk_merchant_round），merchantID 传 0 表示不限商户
	// （只读排障场景，走 idx_round_id）；写路径必须传真实商户，否则会改到别家数据。
	MarkSettled(ctx context.Context, merchantID uint64, roundID string) error
	MarkWinPending(ctx context.Context, merchantID uint64, roundID string, winAmount int64, lastError string) error
	ListStalePending(ctx context.Context, olderThan time.Duration, limit int) ([]*PendingTransaction, error)
	FindByTraceID(ctx context.Context, traceID string) ([]*PendingTransaction, error)
	FindByRoundID(ctx context.Context, merchantID uint64, roundID string) (*PendingTransaction, error)
	MarkDone(ctx context.Context, id uint64) error
	MarkFailed(ctx context.Context, id uint64, lastError string) error
	IncrementRetry(ctx context.Context, id uint64, lastError string) error
}

type defaultPendingTransactionsModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewPendingTransactionsModel(conn sqlx.SqlConn) PendingTransactionsModel {
	return &defaultPendingTransactionsModel{
		conn:  conn,
		table: "`pending_transactions`",
	}
}

func (m *defaultPendingTransactionsModel) Insert(ctx context.Context, data *PendingTransaction) error {
	query := fmt.Sprintf(`
insert into %s (
  trace_id, round_id, merchant_id, merchant_code, user_id, game_code, phase, status,
  bet_amount, win_amount, expected_action, wallet_bet_status, wallet_win_status
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
on duplicate key update
  trace_id = values(trace_id),
  phase = values(phase),
  status = values(status),
  win_amount = values(win_amount),
  expected_action = values(expected_action),
  wallet_win_status = values(wallet_win_status),
  updated_at = current_timestamp(3)
`, m.table)

	status := data.Status
	if status == "" {
		status = PendingTxStatusPending
	}
	return m.execInsert(ctx, query,
		data.TraceID, data.RoundID, data.MerchantID, data.MerchantCode, data.UserID, data.GameCode,
		data.Phase, status, data.BetAmount, data.WinAmount, data.ExpectedAction,
		data.WalletBetStatus, data.WalletWinStatus,
	)
}

func (m *defaultPendingTransactionsModel) execInsert(ctx context.Context, query string, args ...any) error {
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *defaultPendingTransactionsModel) MarkSettled(ctx context.Context, merchantID uint64, roundID string) error {
	where, args := roundScope(merchantID, roundID)
	query := fmt.Sprintf(`
update %s set phase = ?, status = ?, expected_action = ?, wallet_win_status = 'confirmed', last_error = null
where %s
`, m.table, where)
	params := append([]any{PendingTxPhaseSettled, PendingTxStatusDone, PendingTxActionNone}, args...)
	_, err := m.conn.ExecCtx(ctx, query, params...)
	return err
}

func (m *defaultPendingTransactionsModel) MarkWinPending(ctx context.Context, merchantID uint64, roundID string, winAmount int64, lastError string) error {
	where, args := roundScope(merchantID, roundID)
	query := fmt.Sprintf(`
update %s set phase = ?, status = ?, win_amount = ?, expected_action = ?, wallet_win_status = 'failed', last_error = ?
where %s
`, m.table, where)
	params := append([]any{
		PendingTxPhaseWinPending, PendingTxStatusPending, winAmount, PendingTxActionSettleWin, lastError,
	}, args...)
	_, err := m.conn.ExecCtx(ctx, query, params...)
	return err
}

func (m *defaultPendingTransactionsModel) ListStalePending(ctx context.Context, olderThan time.Duration, limit int) ([]*PendingTransaction, error) {
	if limit <= 0 {
		limit = 50
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	query := fmt.Sprintf(`
select * from %s
where status = ? and created_at <= ?
order by created_at asc
limit ?
`, m.table)
	var list []*PendingTransaction
	if err := m.conn.QueryRowsCtx(ctx, &list, query, PendingTxStatusPending, cutoff, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *defaultPendingTransactionsModel) FindByTraceID(ctx context.Context, traceID string) ([]*PendingTransaction, error) {
	query := fmt.Sprintf("select * from %s where trace_id = ? order by created_at desc limit 100", m.table)
	var list []*PendingTransaction
	if err := m.conn.QueryRowsCtx(ctx, &list, query, traceID); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *defaultPendingTransactionsModel) FindByRoundID(ctx context.Context, merchantID uint64, roundID string) (*PendingTransaction, error) {
	where, args := roundScope(merchantID, roundID)
	query := fmt.Sprintf("select * from %s where %s order by id desc limit 1", m.table, where)
	var row PendingTransaction
	err := m.conn.QueryRowCtx(ctx, &row, query, args...)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultPendingTransactionsModel) MarkDone(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("update %s set status = ?, phase = ?, expected_action = ?, last_error = null where id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, PendingTxStatusDone, PendingTxPhaseSettled, PendingTxActionNone, id)
	return err
}

func (m *defaultPendingTransactionsModel) MarkFailed(ctx context.Context, id uint64, lastError string) error {
	query := fmt.Sprintf("update %s set status = ?, last_error = ? where id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, PendingTxStatusFailed, lastError, id)
	return err
}

func (m *defaultPendingTransactionsModel) IncrementRetry(ctx context.Context, id uint64, lastError string) error {
	query := fmt.Sprintf(`
update %s set retry_count = retry_count + 1, last_error = ?, updated_at = current_timestamp(3) where id = ?
`, m.table)
	_, err := m.conn.ExecCtx(ctx, query, lastError, id)
	return err
}
