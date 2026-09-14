package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ I18nMerchantOverridesModel = (*customI18nMerchantOverridesModel)(nil)

type (
	// I18nMerchantOverridesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customI18nMerchantOverridesModel.
	I18nMerchantOverridesModel interface {
		i18nMerchantOverridesModel
	}

	customI18nMerchantOverridesModel struct {
		*defaultI18nMerchantOverridesModel
	}
)

// NewI18nMerchantOverridesModel returns a model for the database table.
func NewI18nMerchantOverridesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) I18nMerchantOverridesModel {
	return &customI18nMerchantOverridesModel{
		defaultI18nMerchantOverridesModel: newI18nMerchantOverridesModel(conn, c, opts...),
	}
}
