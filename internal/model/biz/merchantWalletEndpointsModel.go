package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantWalletEndpointsModel = (*customMerchantWalletEndpointsModel)(nil)

type (
	// MerchantWalletEndpointsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantWalletEndpointsModel.
	MerchantWalletEndpointsModel interface {
		merchantWalletEndpointsModel
	}

	customMerchantWalletEndpointsModel struct {
		*defaultMerchantWalletEndpointsModel
	}
)

// NewMerchantWalletEndpointsModel returns a model for the database table.
func NewMerchantWalletEndpointsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantWalletEndpointsModel {
	return &customMerchantWalletEndpointsModel{
		defaultMerchantWalletEndpointsModel: newMerchantWalletEndpointsModel(conn, c, opts...),
	}
}
