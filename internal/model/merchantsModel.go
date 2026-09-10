package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ MerchantsModel = (*customMerchantsModel)(nil)

type (
	// MerchantsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantsModel.
	MerchantsModel interface {
		merchantsModel
		withSession(session sqlx.Session) MerchantsModel
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
