package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ I18nLanguagesModel = (*customI18nLanguagesModel)(nil)

type (
	// I18nLanguagesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customI18nLanguagesModel.
	I18nLanguagesModel interface {
		i18nLanguagesModel
	}

	customI18nLanguagesModel struct {
		*defaultI18nLanguagesModel
	}
)

// NewI18nLanguagesModel returns a model for the database table.
func NewI18nLanguagesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) I18nLanguagesModel {
	return &customI18nLanguagesModel{
		defaultI18nLanguagesModel: newI18nLanguagesModel(conn, c, opts...),
	}
}
