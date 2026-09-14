package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PromoUserProgressModel = (*customPromoUserProgressModel)(nil)

type (
	// PromoUserProgressModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPromoUserProgressModel.
	PromoUserProgressModel interface {
		promoUserProgressModel
	}

	customPromoUserProgressModel struct {
		*defaultPromoUserProgressModel
	}
)

// NewPromoUserProgressModel returns a model for the database table.
func NewPromoUserProgressModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PromoUserProgressModel {
	return &customPromoUserProgressModel{
		defaultPromoUserProgressModel: newPromoUserProgressModel(conn, c, opts...),
	}
}
