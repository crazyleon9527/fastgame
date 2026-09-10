package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GameConfigsModel = (*customGameConfigsModel)(nil)

type (
	GameConfigsModel interface {
		gameConfigsModel
		withSession(session sqlx.Session) GameConfigsModel
		FindActiveByMerchantGame(ctx context.Context, merchantId uint64, gameCode string) ([]*GameConfigs, error)
	}

	customGameConfigsModel struct {
		*defaultGameConfigsModel
	}
)

func NewGameConfigsModel(conn sqlx.SqlConn) GameConfigsModel {
	return &customGameConfigsModel{
		defaultGameConfigsModel: newGameConfigsModel(conn),
	}
}

func (m *customGameConfigsModel) withSession(session sqlx.Session) GameConfigsModel {
	return NewGameConfigsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customGameConfigsModel) FindActiveByMerchantGame(ctx context.Context, merchantId uint64, gameCode string) ([]*GameConfigs, error) {
	query := fmt.Sprintf("select %s from %s where `merchant_id` = ? and `game_code` = ? and `status` = 1", gameConfigsRows, m.table)
	var resp []*GameConfigs
	err := m.conn.QueryRowsCtx(ctx, &resp, query, merchantId, gameCode)
	return resp, err
}
