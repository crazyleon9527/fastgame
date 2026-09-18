package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GameSessionsModel = (*customGameSessionsModel)(nil)

type (
	// GameSessionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGameSessionsModel.
	GameSessionsModel interface {
		gameSessionsModel
	}

	customGameSessionsModel struct {
		*defaultGameSessionsModel
	}
)

// NewGameSessionsModel returns a model for the database table.
func NewGameSessionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) GameSessionsModel {
	return &customGameSessionsModel{
		defaultGameSessionsModel: newGameSessionsModel(conn, c, opts...),
	}
}
