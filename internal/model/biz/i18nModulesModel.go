package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ I18nModulesModel = (*customI18nModulesModel)(nil)

type (
	// I18nModulesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customI18nModulesModel.
	I18nModulesModel interface {
		i18nModulesModel
	}

	customI18nModulesModel struct {
		*defaultI18nModulesModel
	}
)

// NewI18nModulesModel returns a model for the database table.
func NewI18nModulesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) I18nModulesModel {
	return &customI18nModulesModel{
		defaultI18nModulesModel: newI18nModulesModel(conn, c, opts...),
	}
}
