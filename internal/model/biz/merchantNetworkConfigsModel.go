package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantNetworkConfigsModel = (*customMerchantNetworkConfigsModel)(nil)

type (
	// MerchantNetworkConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantNetworkConfigsModel.
	MerchantNetworkConfigsModel interface {
		merchantNetworkConfigsModel
	}

	customMerchantNetworkConfigsModel struct {
		*defaultMerchantNetworkConfigsModel
	}
)

// NewMerchantNetworkConfigsModel returns a model for the database table.
func NewMerchantNetworkConfigsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantNetworkConfigsModel {
	return &customMerchantNetworkConfigsModel{
		defaultMerchantNetworkConfigsModel: newMerchantNetworkConfigsModel(conn, c, opts...),
	}
}
