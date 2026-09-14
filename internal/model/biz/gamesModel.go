package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GamesModel = (*customGamesModel)(nil)

type (
	// GamesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGamesModel.
	GamesModel interface {
		gamesModel
	}

	customGamesModel struct {
		*defaultGamesModel
	}
)

// NewGamesModel returns a model for the database table.
func NewGamesModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) GamesModel {
	return &customGamesModel{
		defaultGamesModel: newGamesModel(conn, c, opts...),
	}
}
