package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantPromoConfigsModel = (*customMerchantPromoConfigsModel)(nil)

type (
	// MerchantPromoConfigsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantPromoConfigsModel.
	MerchantPromoConfigsModel interface {
		merchantPromoConfigsModel
	}

	customMerchantPromoConfigsModel struct {
		*defaultMerchantPromoConfigsModel
	}
)

// NewMerchantPromoConfigsModel returns a model for the database table.
func NewMerchantPromoConfigsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantPromoConfigsModel {
	return &customMerchantPromoConfigsModel{
		defaultMerchantPromoConfigsModel: newMerchantPromoConfigsModel(conn, c, opts...),
	}
}
