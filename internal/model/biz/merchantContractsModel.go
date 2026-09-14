package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantContractsModel = (*customMerchantContractsModel)(nil)

type (
	// MerchantContractsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantContractsModel.
	MerchantContractsModel interface {
		merchantContractsModel
	}

	customMerchantContractsModel struct {
		*defaultMerchantContractsModel
	}
)

// NewMerchantContractsModel returns a model for the database table.
func NewMerchantContractsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantContractsModel {
	return &customMerchantContractsModel{
		defaultMerchantContractsModel: newMerchantContractsModel(conn, c, opts...),
	}
}
