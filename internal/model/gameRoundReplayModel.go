package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// GameRoundReplay 对应表 game_round_replay：确定性回放输入
type GameRoundReplay struct {
	Id           uint64    `db:"id"`            // 主键
	RoundID      string    `db:"round_id"`      // 局 ID，同时作为 nonce
	MerchantCode string    `db:"merchant_code"` // 商户编码
	UserID       uint64    `db:"user_id"`       // 玩家 ID
	GameCode     string    `db:"game_code"`     // 游戏编码
	ServerSeed   string    `db:"server_seed"`   // 服务端种子（hex）
	ClientSeed   string    `db:"client_seed"`   // 客户端种子
	Nonce        string    `db:"nonce"`         // 局 ID（provably-fair 的 nonce）
	BetAmount    int64     `db:"bet_amount"`    // 下注额，minor units
	SequenceID   uint64    `db:"sequence_id"`   // 会话内单调递增序号
	CreatedAt    time.Time `db:"created_at"`    // 创建时间（UTC）
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
