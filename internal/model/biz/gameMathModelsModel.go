package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GameMathModelsModel = (*customGameMathModelsModel)(nil)

type (
	// GameMathModelsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGameMathModelsModel.
	GameMathModelsModel interface {
		gameMathModelsModel
	}

	customGameMathModelsModel struct {
		*defaultGameMathModelsModel
	}
)

// NewGameMathModelsModel returns a model for the database table.
func NewGameMathModelsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) GameMathModelsModel {
	return &customGameMathModelsModel{
		defaultGameMathModelsModel: newGameMathModelsModel(conn, c, opts...),
	}
}
