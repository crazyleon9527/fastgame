package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PromoMissionDefinitionsModel = (*customPromoMissionDefinitionsModel)(nil)

type (
	// PromoMissionDefinitionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPromoMissionDefinitionsModel.
	PromoMissionDefinitionsModel interface {
		promoMissionDefinitionsModel
	}

	customPromoMissionDefinitionsModel struct {
		*defaultPromoMissionDefinitionsModel
	}
)

// NewPromoMissionDefinitionsModel returns a model for the database table.
func NewPromoMissionDefinitionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PromoMissionDefinitionsModel {
	return &customPromoMissionDefinitionsModel{
		defaultPromoMissionDefinitionsModel: newPromoMissionDefinitionsModel(conn, c, opts...),
	}
}
