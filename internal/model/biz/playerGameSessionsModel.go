package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PlayerGameSessionsModel = (*customPlayerGameSessionsModel)(nil)

type (
	// PlayerGameSessionsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPlayerGameSessionsModel.
	PlayerGameSessionsModel interface {
		playerGameSessionsModel
	}

	customPlayerGameSessionsModel struct {
		*defaultPlayerGameSessionsModel
	}
)

// NewPlayerGameSessionsModel returns a model for the database table.
func NewPlayerGameSessionsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PlayerGameSessionsModel {
	return &customPlayerGameSessionsModel{
		defaultPlayerGameSessionsModel: newPlayerGameSessionsModel(conn, c, opts...),
	}
}
