package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PromoCampaignsModel = (*customPromoCampaignsModel)(nil)

type (
	// PromoCampaignsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPromoCampaignsModel.
	PromoCampaignsModel interface {
		promoCampaignsModel
	}

	customPromoCampaignsModel struct {
		*defaultPromoCampaignsModel
	}
)

// NewPromoCampaignsModel returns a model for the database table.
func NewPromoCampaignsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PromoCampaignsModel {
	return &customPromoCampaignsModel{
		defaultPromoCampaignsModel: newPromoCampaignsModel(conn, c, opts...),
	}
}
