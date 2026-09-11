package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DailySettlement struct {
	Id          uint64    `db:"id"`
	MerchantId  uint64    `db:"merchant_id"`
	SettleDate  time.Time `db:"settle_date"`
	TotalBet    int64     `db:"total_bet"`
	TotalWin    int64     `db:"total_win"`
	TotalRounds uint64    `db:"total_rounds"`
	Status      int64     `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type DailySettlementsModel interface {
	ListPage(ctx context.Context, merchantID uint64, page, pageSize int) ([]*DailySettlement, int64, error)
	FindOne(ctx context.Context, id uint64) (*DailySettlement, error)
	UpsertPending(ctx context.Context, merchantID uint64, settleDate time.Time, totalBet, totalWin int64, totalRounds uint64) error
	Confirm(ctx context.Context, id uint64) error
}

type defaultDailySettlementsModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewDailySettlementsModel(conn sqlx.SqlConn) DailySettlementsModel {
	return &defaultDailySettlementsModel{
		conn:  conn,
		table: "`daily_settlements`",
	}
}

func (m *defaultDailySettlementsModel) ListPage(ctx context.Context, merchantID uint64, page, pageSize int) ([]*DailySettlement, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "1=1"
	args := []any{}
	if merchantID > 0 {
		where += " AND merchant_id = ?"
		args = append(args, merchantID)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		"select id, merchant_id, settle_date, total_bet, total_win, total_rounds, status, created_at, updated_at from %s where %s order by settle_date desc, merchant_id asc limit ? offset ?",
		m.table, where,
	)
	listArgs := append(append([]any{}, args...), pageSize, offset)
	var list []*DailySettlement
	if err := m.conn.QueryRowsCtx(ctx, &list, query, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultDailySettlementsModel) FindOne(ctx context.Context, id uint64) (*DailySettlement, error) {
	query := fmt.Sprintf(
		"select id, merchant_id, settle_date, total_bet, total_win, total_rounds, status, created_at, updated_at from %s where id = ? limit 1",
		m.table,
	)
	var row DailySettlement
	err := m.conn.QueryRowCtx(ctx, &row, query, id)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultDailySettlementsModel) UpsertPending(ctx context.Context, merchantID uint64, settleDate time.Time, totalBet, totalWin int64, totalRounds uint64) error {
	query := fmt.Sprintf(`
		insert into %s (merchant_id, settle_date, total_bet, total_win, total_rounds, status)
		values (?, ?, ?, ?, ?, 0)
		on duplicate key update
			total_bet = if(status = 0, values(total_bet), total_bet),
			total_win = if(status = 0, values(total_win), total_win),
			total_rounds = if(status = 0, values(total_rounds), total_rounds),
			updated_at = current_timestamp(3)
	`, m.table)
	_, err := m.conn.ExecCtx(ctx, query, merchantID, settleDate.Format("2006-01-02"), totalBet, totalWin, totalRounds)
	return err
}

func (m *defaultDailySettlementsModel) Confirm(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("update %s set status = 1 where id = ? and status = 0", m.table)
	result, err := m.conn.ExecCtx(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		row, findErr := m.FindOne(ctx, id)
		if findErr != nil {
			return findErr
		}
		if row.Status == 1 {
			return nil
		}
		return sql.ErrNoRows
	}
	return nil
}
