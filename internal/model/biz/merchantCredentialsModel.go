package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantCredentialsModel = (*customMerchantCredentialsModel)(nil)

type (
	// MerchantCredentialsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantCredentialsModel.
	MerchantCredentialsModel interface {
		merchantCredentialsModel
	}

	customMerchantCredentialsModel struct {
		*defaultMerchantCredentialsModel
	}
)

// NewMerchantCredentialsModel returns a model for the database table.
func NewMerchantCredentialsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantCredentialsModel {
	return &customMerchantCredentialsModel{
		defaultMerchantCredentialsModel: newMerchantCredentialsModel(conn, c, opts...),
	}
}
