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

// WalletPendingOp 对应表 wallet_pending_ops：钱包待对账
type WalletPendingOp struct {
	Id           uint64         `db:"id"` // 主键
	RoundID      string         `db:"round_id"` // 局 ID，同时作为 nonce
	MerchantID   uint64         `db:"merchant_id"` // 商户 ID（merchants.id），唯一键前导列
	MerchantCode string         `db:"merchant_code"` // 商户编码
	UserID       string         `db:"user_id"` // 玩家 ID
	OpType       string         `db:"op_type"` // 操作类型：win_failed / win_timeout / rollback
	BetAmount    int64          `db:"bet_amount"` // 下注额，minor units
	WinAmount    int64          `db:"win_amount"` // 派彩额，minor units
	Status       string         `db:"status"` // 状态：pending / done / failed
	RetryCount   int64          `db:"retry_count"` // 已重试次数
	LastError    sql.NullString `db:"last_error"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

type WalletPendingOpsModel interface {
	Insert(ctx context.Context, data *WalletPendingOp) (sql.Result, error)
	// FindByRoundOp 按 (merchant_id, round_id, op_type) 定位待对账操作。
	// round_id 只在商户内唯一（uk_merchant_round_op），merchantID 传 0 表示不限商户
	// （只读排障场景）；写路径必须传真实商户，否则会读到别家的记录。
	FindByRoundOp(ctx context.Context, merchantID uint64, roundID, opType string) (*WalletPendingOp, error)
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
insert into %s (round_id, merchant_id, merchant_code, user_id, op_type, bet_amount, win_amount, status, retry_count, last_error)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		data.RoundID, data.MerchantID, data.MerchantCode, data.UserID, data.OpType,
		data.BetAmount, data.WinAmount, status, data.RetryCount, data.LastError,
	)
}

func (m *defaultWalletPendingOpsModel) FindByRoundOp(ctx context.Context, merchantID uint64, roundID, opType string) (*WalletPendingOp, error) {
	where, args := roundScope(merchantID, roundID)
	query := fmt.Sprintf("select * from %s where %s and op_type = ? order by id desc limit 1", m.table, where)
	var resp WalletPendingOp
	err := m.conn.QueryRowCtx(ctx, &resp, query, append(args, opType)...)
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
