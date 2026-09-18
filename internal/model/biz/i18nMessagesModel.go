package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ I18nMessagesModel = (*customI18nMessagesModel)(nil)

type (
	// I18nMessagesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customI18nMessagesModel.
	I18nMessagesModel interface {
		i18nMessagesModel
	}

	customI18nMessagesModel struct {
		*defaultI18nMessagesModel
	}
)

// NewI18nMessagesModel returns a model for the database table.
func NewI18nMessagesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) I18nMessagesModel {
	return &customI18nMessagesModel{
		defaultI18nMessagesModel: newI18nMessagesModel(conn, c, opts...),
	}
}
