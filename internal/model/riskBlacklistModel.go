package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	BlacklistTypeIP       = "ip"
	BlacklistTypeUserID   = "user_id"
	BlacklistTypeMerchant = "merchant"
)

type RiskBlacklist struct {
	Id        uint64       `db:"id"`
	ListType  string       `db:"list_type"`
	ListValue string       `db:"list_value"`
	Reason    string       `db:"reason"`
	Status    int64        `db:"status"`
	ExpiresAt sql.NullTime `db:"expires_at"`
	CreatedAt time.Time    `db:"created_at"`
	UpdatedAt time.Time    `db:"updated_at"`
}

type RiskBlacklistModel interface {
	Insert(ctx context.Context, data *RiskBlacklist) (sql.Result, error)
	FindOne(ctx context.Context, id uint64) (*RiskBlacklist, error)
	FindActiveByTypeValue(ctx context.Context, listType, listValue string) (*RiskBlacklist, error)
	FindByTypeValue(ctx context.Context, listType, listValue string) (*RiskBlacklist, error)
	ListPage(ctx context.Context, listType string, page, pageSize int) ([]*RiskBlacklist, int64, error)
	ListAllActive(ctx context.Context) ([]*RiskBlacklist, error)
	UpdateStatus(ctx context.Context, id uint64, status int64) error
	Upsert(ctx context.Context, data *RiskBlacklist) (uint64, error)
}

type defaultRiskBlacklistModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewRiskBlacklistModel(conn sqlx.SqlConn) RiskBlacklistModel {
	return &defaultRiskBlacklistModel{
		conn:  conn,
		table: "`risk_blacklist`",
	}
}

func (m *defaultRiskBlacklistModel) Insert(ctx context.Context, data *RiskBlacklist) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (`list_type`, `list_value`, `reason`, `status`, `expires_at`) values (?, ?, ?, ?, ?)",
		m.table,
	)
	return m.conn.ExecCtx(ctx, query, data.ListType, data.ListValue, data.Reason, data.Status, data.ExpiresAt)
}

func (m *defaultRiskBlacklistModel) FindOne(ctx context.Context, id uint64) (*RiskBlacklist, error) {
	query := fmt.Sprintf("select * from %s where `id` = ? limit 1", m.table)
	var resp RiskBlacklist
	err := m.conn.QueryRowCtx(ctx, &resp, query, id)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultRiskBlacklistModel) FindByTypeValue(ctx context.Context, listType, listValue string) (*RiskBlacklist, error) {
	query := fmt.Sprintf("select * from %s where `list_type` = ? and `list_value` = ? limit 1", m.table)
	var resp RiskBlacklist
	err := m.conn.QueryRowCtx(ctx, &resp, query, listType, listValue)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultRiskBlacklistModel) FindActiveByTypeValue(ctx context.Context, listType, listValue string) (*RiskBlacklist, error) {
	query := fmt.Sprintf(
		"select * from %s where `list_type` = ? and `list_value` = ? and `status` = 1 and (`expires_at` is null or `expires_at` > utc_timestamp(3)) limit 1",
		m.table,
	)
	var resp RiskBlacklist
	err := m.conn.QueryRowCtx(ctx, &resp, query, listType, listValue)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultRiskBlacklistModel) ListPage(ctx context.Context, listType string, page, pageSize int) ([]*RiskBlacklist, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "where 1=1"
	args := make([]any, 0, 2)
	if listType != "" {
		where += " and `list_type` = ?"
		args = append(args, listType)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s %s", m.table, where)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("select * from %s %s order by id desc limit ? offset ?", m.table, where)
	args = append(args, pageSize, offset)
	var list []*RiskBlacklist
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultRiskBlacklistModel) ListAllActive(ctx context.Context) ([]*RiskBlacklist, error) {
	query := fmt.Sprintf(
		"select * from %s where `status` = 1 and (`expires_at` is null or `expires_at` > utc_timestamp(3)) order by id asc",
		m.table,
	)
	var list []*RiskBlacklist
	if err := m.conn.QueryRowsCtx(ctx, &list, query); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *defaultRiskBlacklistModel) UpdateStatus(ctx context.Context, id uint64, status int64) error {
	query := fmt.Sprintf("update %s set `status` = ? where `id` = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

func (m *defaultRiskBlacklistModel) Upsert(ctx context.Context, data *RiskBlacklist) (uint64, error) {
	query := fmt.Sprintf(`
insert into %s (list_type, list_value, reason, status, expires_at)
values (?, ?, ?, ?, ?)
on duplicate key update reason = values(reason), status = values(status), expires_at = values(expires_at)`,
		m.table,
	)
	result, err := m.conn.ExecCtx(ctx, query, data.ListType, data.ListValue, data.Reason, data.Status, data.ExpiresAt)
	if err != nil {
		return 0, err
	}
	if id, err := result.LastInsertId(); err == nil && id > 0 {
		return uint64(id), nil
	}
	existing, err := m.FindByTypeValue(ctx, data.ListType, data.ListValue)
	if err != nil {
		return 0, err
	}
	return existing.Id, nil
}
