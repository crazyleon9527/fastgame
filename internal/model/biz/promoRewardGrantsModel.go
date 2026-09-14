package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PromoRewardGrantsModel = (*customPromoRewardGrantsModel)(nil)

type (
	// PromoRewardGrantsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPromoRewardGrantsModel.
	PromoRewardGrantsModel interface {
		promoRewardGrantsModel
	}

	customPromoRewardGrantsModel struct {
		*defaultPromoRewardGrantsModel
	}
)

// NewPromoRewardGrantsModel returns a model for the database table.
func NewPromoRewardGrantsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PromoRewardGrantsModel {
	return &customPromoRewardGrantsModel{
		defaultPromoRewardGrantsModel: newPromoRewardGrantsModel(conn, c, opts...),
	}
}
