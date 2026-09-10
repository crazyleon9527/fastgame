package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	PendingOpWinFailed   = "win_failed"
	PendingOpWinTimeout  = "win_timeout"
	PendingOpRollback    = "rollback"
	PendingStatusPending = "pending"
	PendingStatusDone    = "done"
	PendingStatusFailed  = "failed"
)

type WalletPendingOp struct {
	Id           uint64         `db:"id"`
	RoundID      string         `db:"round_id"`
	MerchantCode string         `db:"merchant_code"`
	UserID       uint64         `db:"user_id"`
	OpType       string         `db:"op_type"`
	BetAmount    float64        `db:"bet_amount"`
	WinAmount    float64        `db:"win_amount"`
	Status       string         `db:"status"`
	RetryCount   int64          `db:"retry_count"`
	LastError    sql.NullString `db:"last_error"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

type WalletPendingOpsModel interface {
	Insert(ctx context.Context, data *WalletPendingOp) (sql.Result, error)
	FindByRoundOp(ctx context.Context, roundID, opType string) (*WalletPendingOp, error)
	ListPending(ctx context.Context, limit int) ([]*WalletPendingOp, error)
	MarkDone(ctx context.Context, id uint64) error
	MarkFailed(ctx context.Context, id uint64, lastError string) error
	IncrementRetry(ctx context.Context, id uint64, lastError string) error
}

type defaultWalletPendingOpsModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewWalletPendingOpsModel(conn sqlx.SqlConn) WalletPendingOpsModel {
	return &defaultWalletPendingOpsModel{
		conn:  conn,
		table: "`wallet_pending_ops`",
	}
}

func (m *defaultWalletPendingOpsModel) Insert(ctx context.Context, data *WalletPendingOp) (sql.Result, error) {
	query := fmt.Sprintf(`
insert into %s (round_id, merchant_code, user_id, op_type, bet_amount, win_amount, status, retry_count, last_error)
values (?, ?, ?, ?, ?, ?, ?, ?, ?)
on duplicate key update
  status = values(status),
  win_amount = values(win_amount),
  last_error = values(last_error),
  updated_at = current_timestamp(3)`,
		m.table,
	)
	status := data.Status
	if status == "" {
		status = PendingStatusPending
	}
	return m.conn.ExecCtx(ctx, query,
		data.RoundID, data.MerchantCode, data.UserID, data.OpType,
		data.BetAmount, data.WinAmount, status, data.RetryCount, data.LastError,
	)
}

func (m *defaultWalletPendingOpsModel) FindByRoundOp(ctx context.Context, roundID, opType string) (*WalletPendingOp, error) {
	query := fmt.Sprintf("select * from %s where round_id = ? and op_type = ? limit 1", m.table)
	var resp WalletPendingOp
	err := m.conn.QueryRowCtx(ctx, &resp, query, roundID, opType)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultWalletPendingOpsModel) ListPending(ctx context.Context, limit int) ([]*WalletPendingOp, error) {
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf(
		"select * from %s where status = ? order by updated_at asc limit ?",
		m.table,
	)
	var list []*WalletPendingOp
	if err := m.conn.QueryRowsCtx(ctx, &list, query, PendingStatusPending, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *defaultWalletPendingOpsModel) MarkDone(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("update %s set status = ?, last_error = null where id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, PendingStatusDone, id)
	return err
}

func (m *defaultWalletPendingOpsModel) MarkFailed(ctx context.Context, id uint64, lastError string) error {
	query := fmt.Sprintf("update %s set status = ?, last_error = ? where id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, PendingStatusFailed, lastError, id)
	return err
}

func (m *defaultWalletPendingOpsModel) IncrementRetry(ctx context.Context, id uint64, lastError string) error {
	query := fmt.Sprintf(
		"update %s set retry_count = retry_count + 1, last_error = ?, updated_at = current_timestamp(3) where id = ?",
		m.table,
	)
	_, err := m.conn.ExecCtx(ctx, query, lastError, id)
	return err
}
