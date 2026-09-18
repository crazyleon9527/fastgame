package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GameTransactionsModel = (*customGameTransactionsModel)(nil)

type (
	// GameTransactionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGameTransactionsModel.
	GameTransactionsModel interface {
		gameTransactionsModel
	}

	customGameTransactionsModel struct {
		*defaultGameTransactionsModel
	}
)

// NewGameTransactionsModel returns a model for the database table.
func NewGameTransactionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) GameTransactionsModel {
	return &customGameTransactionsModel{
		defaultGameTransactionsModel: newGameTransactionsModel(conn, c, opts...),
	}
}
