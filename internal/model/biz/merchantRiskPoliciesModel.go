package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantRiskPoliciesModel = (*customMerchantRiskPoliciesModel)(nil)

type (
	// MerchantRiskPoliciesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantRiskPoliciesModel.
	MerchantRiskPoliciesModel interface {
		merchantRiskPoliciesModel
	}

	customMerchantRiskPoliciesModel struct {
		*defaultMerchantRiskPoliciesModel
	}
)

// NewMerchantRiskPoliciesModel returns a model for the database table.
func NewMerchantRiskPoliciesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantRiskPoliciesModel {
	return &customMerchantRiskPoliciesModel{
		defaultMerchantRiskPoliciesModel: newMerchantRiskPoliciesModel(conn, c, opts...),
	}
}
