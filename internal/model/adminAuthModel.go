package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type AdminAuthRecord struct {
	Id           uint64         `db:"id"`
	Username     string         `db:"username"`
	PasswordHash string         `db:"password_hash"`
	RoleId       uint64         `db:"role_id"`
	Status       int64          `db:"status"`
	TotpSecret   sql.NullString `db:"totp_secret"`
	TotpEnabled  int64          `db:"totp_enabled"`
}

type AdminAuthModel interface {
	FindByUsername(ctx context.Context, username string) (*AdminAuthRecord, error)
}

type defaultAdminAuthModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewAdminAuthModel(conn sqlx.SqlConn) AdminAuthModel {
	return &defaultAdminAuthModel{conn: conn, table: "`admin_users`"}
}

func (m *defaultAdminAuthModel) FindByUsername(ctx context.Context, username string) (*AdminAuthRecord, error) {
	query := fmt.Sprintf(`
select id, username, password_hash, role_id, status, totp_secret, totp_enabled
from %s where username = ? limit 1
`, m.table)
	var row AdminAuthRecord
	err := m.conn.QueryRowCtx(ctx, &row, query, username)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
