package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type GameRoundReplay struct {
	Id           uint64    `db:"id"`
	RoundID      string    `db:"round_id"`
	MerchantCode string    `db:"merchant_code"`
	UserID       uint64    `db:"user_id"`
	GameCode     string    `db:"game_code"`
	ServerSeed   string    `db:"server_seed"`
	ClientSeed   string    `db:"client_seed"`
	Nonce        string    `db:"nonce"`
	BetAmount    int64     `db:"bet_amount"`
	SequenceID   uint64    `db:"sequence_id"`
	CreatedAt    time.Time `db:"created_at"`
}

type GameRoundReplayModel interface {
	Insert(ctx context.Context, data *GameRoundReplay) error
	FindByRoundID(ctx context.Context, roundID string) (*GameRoundReplay, error)
}

type defaultGameRoundReplayModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewGameRoundReplayModel(conn sqlx.SqlConn) GameRoundReplayModel {
	return &defaultGameRoundReplayModel{
		conn:  conn,
		table: "`game_round_replay`",
	}
}

func (m *defaultGameRoundReplayModel) Insert(ctx context.Context, data *GameRoundReplay) error {
	query := fmt.Sprintf(`
insert into %s (round_id, merchant_code, user_id, game_code, server_seed, client_seed, nonce, bet_amount, sequence_id)
values (?, ?, ?, ?, ?, ?, ?, ?, ?)
on duplicate key update round_id = round_id
`, m.table)
	_, err := m.conn.ExecCtx(ctx, query,
		data.RoundID, data.MerchantCode, data.UserID, data.GameCode,
		data.ServerSeed, data.ClientSeed, data.Nonce, data.BetAmount, data.SequenceID,
	)
	return err
}

func (m *defaultGameRoundReplayModel) FindByRoundID(ctx context.Context, roundID string) (*GameRoundReplay, error) {
	query := fmt.Sprintf(`
select id, round_id, merchant_code, user_id, game_code, server_seed, client_seed, nonce, bet_amount, sequence_id, created_at
from %s where round_id = ? limit 1
`, m.table)
	var row GameRoundReplay
	err := m.conn.QueryRowCtx(ctx, &row, query, roundID)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
