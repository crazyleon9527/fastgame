package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AdminUsersModel = (*customAdminUsersModel)(nil)

type AdminUserListRow struct {
	Id       uint64 `db:"id"`
	Username string `db:"username"`
	RoleId   uint64 `db:"role_id"`
	RoleName string `db:"role_name"`
	Status   int64  `db:"status"`
}

type (
	// AdminUsersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAdminUsersModel.
	AdminUsersModel interface {
		adminUsersModel
		withSession(session sqlx.Session) AdminUsersModel
		ListPage(ctx context.Context, page, pageSize int) ([]AdminUserListRow, int64, error)
	}

	customAdminUsersModel struct {
		*defaultAdminUsersModel
	}
)

// NewAdminUsersModel returns a model for the database table.
func NewAdminUsersModel(conn sqlx.SqlConn) AdminUsersModel {
	return &customAdminUsersModel{
		defaultAdminUsersModel: newAdminUsersModel(conn),
	}
}

func (m *customAdminUsersModel) withSession(session sqlx.Session) AdminUsersModel {
	return NewAdminUsersModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAdminUsersModel) ListPage(ctx context.Context, page, pageSize int) ([]AdminUserListRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	countQ := fmt.Sprintf("select count(*) from %s", m.table)
	if err := m.conn.QueryRowCtx(ctx, &total, countQ); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
select u.id, u.username, u.role_id, r.name as role_name, u.status
from %s u join roles r on u.role_id = r.id
order by u.id asc limit ? offset ?`, m.table)
	var rows []AdminUserListRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, pageSize, offset); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
