package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PromoTournamentRulesModel = (*customPromoTournamentRulesModel)(nil)

type (
	// PromoTournamentRulesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPromoTournamentRulesModel.
	PromoTournamentRulesModel interface {
		promoTournamentRulesModel
	}

	customPromoTournamentRulesModel struct {
		*defaultPromoTournamentRulesModel
	}
)

// NewPromoTournamentRulesModel returns a model for the database table.
func NewPromoTournamentRulesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PromoTournamentRulesModel {
	return &customPromoTournamentRulesModel{
		defaultPromoTournamentRulesModel: newPromoTournamentRulesModel(conn, c, opts...),
	}
}
