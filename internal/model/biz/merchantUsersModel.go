package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantUsersModel = (*customMerchantUsersModel)(nil)

type (
	// MerchantUsersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantUsersModel.
	MerchantUsersModel interface {
		merchantUsersModel
	}

	customMerchantUsersModel struct {
		*defaultMerchantUsersModel
	}
)

// NewMerchantUsersModel returns a model for the database table.
func NewMerchantUsersModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantUsersModel {
	return &customMerchantUsersModel{
		defaultMerchantUsersModel: newMerchantUsersModel(conn, c, opts...),
	}
}
