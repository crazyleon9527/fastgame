package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SystemExchangeRatesModel = (*customSystemExchangeRatesModel)(nil)

type (
	// SystemExchangeRatesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSystemExchangeRatesModel.
	SystemExchangeRatesModel interface {
		systemExchangeRatesModel
	}

	customSystemExchangeRatesModel struct {
		*defaultSystemExchangeRatesModel
	}
)

// NewSystemExchangeRatesModel returns a model for the database table.
func NewSystemExchangeRatesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SystemExchangeRatesModel {
	return &customSystemExchangeRatesModel{
		defaultSystemExchangeRatesModel: newSystemExchangeRatesModel(conn, c, opts...),
	}
}
