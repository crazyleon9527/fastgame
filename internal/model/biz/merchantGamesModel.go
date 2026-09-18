package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantGamesModel = (*customMerchantGamesModel)(nil)

type (
	// MerchantGamesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantGamesModel.
	MerchantGamesModel interface {
		merchantGamesModel
	}

	customMerchantGamesModel struct {
		*defaultMerchantGamesModel
	}
)

// NewMerchantGamesModel returns a model for the database table.
func NewMerchantGamesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantGamesModel {
	return &customMerchantGamesModel{
		defaultMerchantGamesModel: newMerchantGamesModel(conn, c, opts...),
	}
}
