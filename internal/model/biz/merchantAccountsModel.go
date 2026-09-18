package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantAccountsModel = (*customMerchantAccountsModel)(nil)

type (
	// MerchantAccountsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantAccountsModel.
	MerchantAccountsModel interface {
		merchantAccountsModel
	}

	customMerchantAccountsModel struct {
		*defaultMerchantAccountsModel
	}
)

// NewMerchantAccountsModel returns a model for the database table.
func NewMerchantAccountsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantAccountsModel {
	return &customMerchantAccountsModel{
		defaultMerchantAccountsModel: newMerchantAccountsModel(conn, c, opts...),
	}
}
