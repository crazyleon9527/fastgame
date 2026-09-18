package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GameRoundsModel = (*customGameRoundsModel)(nil)

type (
	// GameRoundsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGameRoundsModel.
	GameRoundsModel interface {
		gameRoundsModel
	}

	customGameRoundsModel struct {
		*defaultGameRoundsModel
	}
)

// NewGameRoundsModel returns a model for the database table.
func NewGameRoundsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) GameRoundsModel {
	return &customGameRoundsModel{
		defaultGameRoundsModel: newGameRoundsModel(conn, c, opts...),
	}
}
