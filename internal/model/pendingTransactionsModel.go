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

type PendingTransaction struct {
	Id              uint64         `db:"id"`
	TraceID         string         `db:"trace_id"`
	RoundID         string         `db:"round_id"`
	MerchantCode    string         `db:"merchant_code"`
	UserID          uint64         `db:"user_id"`
	GameCode        string         `db:"game_code"`
	Phase           string         `db:"phase"`
	Status          string         `db:"status"`
	BetAmount       float64        `db:"bet_amount"`
	WinAmount       float64        `db:"win_amount"`
	ExpectedAction  string         `db:"expected_action"`
	WalletBetStatus string         `db:"wallet_bet_status"`
	WalletWinStatus string         `db:"wallet_win_status"`
	RetryCount      int64          `db:"retry_count"`
	LastError       sql.NullString `db:"last_error"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

type PendingTransactionsModel interface {
	Insert(ctx context.Context, data *PendingTransaction) error
	MarkSettled(ctx context.Context, roundID string) error
	MarkWinPending(ctx context.Context, roundID string, winAmount float64, lastError string) error
	ListStalePending(ctx context.Context, olderThan time.Duration, limit int) ([]*PendingTransaction, error)
	FindByTraceID(ctx context.Context, traceID string) ([]*PendingTransaction, error)
	FindByRoundID(ctx context.Context, roundID string) (*PendingTransaction, error)
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
  trace_id, round_id, merchant_code, user_id, game_code, phase, status,
  bet_amount, win_amount, expected_action, wallet_bet_status, wallet_win_status
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		data.TraceID, data.RoundID, data.MerchantCode, data.UserID, data.GameCode,
		data.Phase, status, data.BetAmount, data.WinAmount, data.ExpectedAction,
		data.WalletBetStatus, data.WalletWinStatus,
	)
}

func (m *defaultPendingTransactionsModel) execInsert(ctx context.Context, query string, args ...any) error {
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *defaultPendingTransactionsModel) MarkSettled(ctx context.Context, roundID string) error {
	query := fmt.Sprintf(`
update %s set phase = ?, status = ?, expected_action = ?, wallet_win_status = 'confirmed', last_error = null
where round_id = ?
`, m.table)
	_, err := m.conn.ExecCtx(ctx, query, PendingTxPhaseSettled, PendingTxStatusDone, PendingTxActionNone, roundID)
	return err
}

func (m *defaultPendingTransactionsModel) MarkWinPending(ctx context.Context, roundID string, winAmount float64, lastError string) error {
	query := fmt.Sprintf(`
update %s set phase = ?, status = ?, win_amount = ?, expected_action = ?, wallet_win_status = 'failed', last_error = ?
where round_id = ?
`, m.table)
	_, err := m.conn.ExecCtx(ctx, query,
		PendingTxPhaseWinPending, PendingTxStatusPending, winAmount, PendingTxActionSettleWin, lastError, roundID,
	)
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

func (m *defaultPendingTransactionsModel) FindByRoundID(ctx context.Context, roundID string) (*PendingTransaction, error) {
	query := fmt.Sprintf("select * from %s where round_id = ? limit 1", m.table)
	var row PendingTransaction
	err := m.conn.QueryRowCtx(ctx, &row, query, roundID)
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
