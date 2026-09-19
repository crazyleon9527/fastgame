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
	MerchantID   uint64    `db:"merchant_id"`   // 商户 ID（merchants.id），唯一键前导列
	MerchantCode string    `db:"merchant_code"` // 商户编码
	UserID       string    `db:"user_id"`       // 玩家 ID
	GameCode     string    `db:"game_code"`     // 游戏编码
	ServerSeed   string    `db:"server_seed"`   // 服务端种子（hex）
	ClientSeed   string    `db:"client_seed"`   // 客户端种子
	Nonce        string    `db:"nonce"`         // 局 ID（provably-fair 的 nonce）
	BetAmount    int64     `db:"bet_amount"`    // 下注额，minor units
	SequenceID   uint64    `db:"sequence_id"`   // 会话内单调递增序号
	CreatedAt    time.Time `db:"created_at"`
}

type GameRoundReplayModel interface {
	Insert(ctx context.Context, data *GameRoundReplay) error
	// FindByRoundID 按 (merchant_id, round_id) 定位回放记录。
	// round_id 只在商户内唯一（uk_merchant_round），merchantID 传 0 表示不限商户
	// （公开回放/后台排障这类拿不到商户上下文的只读场景，走 idx_round_id）。
	FindByRoundID(ctx context.Context, merchantID uint64, roundID string) (*GameRoundReplay, error)
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
insert into %s (round_id, merchant_id, merchant_code, user_id, game_code, server_seed, client_seed, nonce, bet_amount, sequence_id)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
on duplicate key update round_id = round_id
`, m.table)
	_, err := m.conn.ExecCtx(ctx, query,
		data.RoundID, data.MerchantID, data.MerchantCode, data.UserID, data.GameCode,
		data.ServerSeed, data.ClientSeed, data.Nonce, data.BetAmount, data.SequenceID,
	)
	return err
}

func (m *defaultGameRoundReplayModel) FindByRoundID(ctx context.Context, merchantID uint64, roundID string) (*GameRoundReplay, error) {
	where, args := roundScope(merchantID, roundID)
	query := fmt.Sprintf(`
select id, round_id, merchant_id, merchant_code, user_id, game_code, server_seed, client_seed, nonce, bet_amount, sequence_id, created_at
from %s where %s order by id desc limit 1
`, m.table, where)
	var row GameRoundReplay
	err := m.conn.QueryRowCtx(ctx, &row, query, args...)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
