package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantReconciliationDiffsModel = (*customMerchantReconciliationDiffsModel)(nil)

type (
	// MerchantReconciliationDiffsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantReconciliationDiffsModel.
	MerchantReconciliationDiffsModel interface {
		merchantReconciliationDiffsModel
	}

	customMerchantReconciliationDiffsModel struct {
		*defaultMerchantReconciliationDiffsModel
	}
)

// NewMerchantReconciliationDiffsModel returns a model for the database table.
func NewMerchantReconciliationDiffsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantReconciliationDiffsModel {
	return &customMerchantReconciliationDiffsModel{
		defaultMerchantReconciliationDiffsModel: newMerchantReconciliationDiffsModel(conn, c, opts...),
	}
}
