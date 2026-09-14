package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantFinancialPeriodsModel = (*customMerchantFinancialPeriodsModel)(nil)

type (
	// MerchantFinancialPeriodsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantFinancialPeriodsModel.
	MerchantFinancialPeriodsModel interface {
		merchantFinancialPeriodsModel
	}

	customMerchantFinancialPeriodsModel struct {
		*defaultMerchantFinancialPeriodsModel
	}
)

// NewMerchantFinancialPeriodsModel returns a model for the database table.
func NewMerchantFinancialPeriodsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantFinancialPeriodsModel {
	return &customMerchantFinancialPeriodsModel{
		defaultMerchantFinancialPeriodsModel: newMerchantFinancialPeriodsModel(conn, c, opts...),
	}
}
