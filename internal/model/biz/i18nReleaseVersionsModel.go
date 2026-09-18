package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ I18nReleaseVersionsModel = (*customI18nReleaseVersionsModel)(nil)

type (
	// I18nReleaseVersionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customI18nReleaseVersionsModel.
	I18nReleaseVersionsModel interface {
		i18nReleaseVersionsModel
	}

	customI18nReleaseVersionsModel struct {
		*defaultI18nReleaseVersionsModel
	}
)

// NewI18nReleaseVersionsModel returns a model for the database table.
func NewI18nReleaseVersionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) I18nReleaseVersionsModel {
	return &customI18nReleaseVersionsModel{
		defaultI18nReleaseVersionsModel: newI18nReleaseVersionsModel(conn, c, opts...),
	}
}
