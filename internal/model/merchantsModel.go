package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantsModel = (*customMerchantsModel)(nil)

type (
	// MerchantsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantsModel.
	MerchantsModel interface {
		merchantsModel
		withSession(session sqlx.Session) MerchantsModel
		ListPage(ctx context.Context, page, pageSize int) ([]*Merchants, int64, error)
	}

	customMerchantsModel struct {
		*defaultMerchantsModel
	}
)

// NewMerchantsModel returns a model for the database table.
func NewMerchantsModel(conn sqlx.SqlConn) MerchantsModel {
	return &customMerchantsModel{
		defaultMerchantsModel: newMerchantsModel(conn),
	}
}

func (m *customMerchantsModel) withSession(session sqlx.Session) MerchantsModel {
	return NewMerchantsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customMerchantsModel) ListPage(ctx context.Context, page, pageSize int) ([]*Merchants, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s", m.table)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("select %s from %s order by id desc limit ? offset ?", merchantsRows, m.table)
	var list []*Merchants
	if err := m.conn.QueryRowsCtx(ctx, &list, query, pageSize, offset); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
