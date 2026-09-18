package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// RoleRow 对应表 games：平台游戏目录
type RoleRow struct {
	Id          uint64 `db:"id"`          // 主键
	Name        string `db:"name"`        // 名称
	Description string `db:"description"` // 说明
}

type RolesModel interface {
	FindNameByID(ctx context.Context, id uint64) (string, error)
	ListAll(ctx context.Context) ([]RoleRow, error)
}

type defaultRolesModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewRolesModel(conn sqlx.SqlConn) RolesModel {
	return &defaultRolesModel{conn: conn, table: "`roles`"}
}

func (m *defaultRolesModel) ListAll(ctx context.Context) ([]RoleRow, error) {
	query := fmt.Sprintf("select id, name, description from %s order by id asc", m.table)
	var rows []RoleRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *defaultRolesModel) FindNameByID(ctx context.Context, id uint64) (string, error) {
	query := fmt.Sprintf("select name from %s where id = ? limit 1", m.table)
	var name string
	err := m.conn.QueryRowCtx(ctx, &name, query, id)
	switch err {
	case nil:
		return name, nil
	case sqlx.ErrNotFound:
		return "", ErrNotFound
	default:
		return "", err
	}
}
