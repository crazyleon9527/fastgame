package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type GameCategory struct {
	Id        uint64 `db:"id"`
	Code      string `db:"code"`
	Name      string `db:"name"`
	SortOrder int64  `db:"sort_order"`
	Status    int64  `db:"status"`
}

type PlatformGame struct {
	Id            uint64         `db:"id"`
	GameCode      string         `db:"game_code"`
	Name          string         `db:"name"`
	CategoryId    sql.NullInt64  `db:"category_id"`
	CategoryCode  string         `db:"category_code"`
	CategoryName  string         `db:"category_name"`
	GameType      string         `db:"game_type"`
	DefaultRtpPpm int64          `db:"default_rtp_ppm"`
	Volatility    sql.NullString `db:"volatility"`
	MinBetMinor   int64          `db:"min_bet_minor"`
	MaxBetMinor   int64          `db:"max_bet_minor"`
	ClientVersion sql.NullString `db:"client_version"`
	ThumbnailUrl  sql.NullString `db:"thumbnail_url"`
	Status        int64          `db:"status"`
	LaunchedAt    sql.NullTime   `db:"launched_at"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

type GameRtpTier struct {
	Id           uint64 `db:"id"`
	GameId       uint64 `db:"game_id"`
	TierCode     string `db:"tier_code"`
	TargetRtpPpm int64  `db:"target_rtp_ppm"`
	ParSheetRef  string `db:"par_sheet_ref"`
	Weight       int64  `db:"weight"`
	Status       int64  `db:"status"`
}

type MerchantGameRow struct {
	Id            uint64         `db:"id"`
	MerchantId    uint64         `db:"merchant_id"`
	MerchantCode  string         `db:"merchant_code"`
	GameId        uint64         `db:"game_id"`
	GameCode      string         `db:"game_code"`
	GameName      string         `db:"game_name"`
	GameType      string         `db:"game_type"`
	GameStatus    int64          `db:"game_status"`
	RtpTierCode   sql.NullString `db:"rtp_tier_code"`
	MinBetMinor   sql.NullInt64  `db:"min_bet_minor"`
	MaxBetMinor   sql.NullInt64  `db:"max_bet_minor"`
	SortOrder     int64          `db:"sort_order"`
	Status        int64          `db:"status"`
	OpenedAt      sql.NullTime   `db:"opened_at"`
	DefaultRtpPpm int64          `db:"default_rtp_ppm"`
}

type PlatformGamesModel interface {
	ListCategories(ctx context.Context) ([]GameCategory, error)
	ListGamesPage(ctx context.Context, page, pageSize int, gameType string) ([]*PlatformGame, int64, error)
	FindGame(ctx context.Context, id uint64) (*PlatformGame, error)
	FindGameByCode(ctx context.Context, code string) (*PlatformGame, error)
	InsertGame(ctx context.Context, g *PlatformGame) (uint64, error)
	UpdateGame(ctx context.Context, g *PlatformGame) error
	ListRtpTiers(ctx context.Context, gameID uint64) ([]GameRtpTier, error)
	InsertRtpTier(ctx context.Context, tier *GameRtpTier) error

	ListMerchantGamesPage(ctx context.Context, merchantID uint64, page, pageSize int) ([]*MerchantGameRow, int64, error)
	FindMerchantGame(ctx context.Context, id uint64) (*MerchantGameRow, error)
	MerchantGameExists(ctx context.Context, merchantID, gameID uint64) (bool, error)
	InsertMerchantGame(ctx context.Context, row *MerchantGameRow) (uint64, error)
	UpdateMerchantGame(ctx context.Context, id uint64, rtpTierCode string, minBet, maxBet sql.NullInt64, sortOrder, status int64) error
	DeleteMerchantGame(ctx context.Context, id uint64) error
}

type defaultPlatformGamesModel struct {
	conn sqlx.SqlConn
}

func NewPlatformGamesModel(conn sqlx.SqlConn) PlatformGamesModel {
	return &defaultPlatformGamesModel{conn: conn}
}

const gameListSelect = `
select g.id, g.game_code, g.name, g.category_id,
  coalesce(gc.code, '') as category_code,
  coalesce(gc.name, '') as category_name,
  g.game_type, g.default_rtp_ppm, g.volatility,
  g.min_bet_minor, g.max_bet_minor, g.client_version, g.thumbnail_url,
  g.status, g.launched_at, g.created_at, g.updated_at
from games g
left join game_categories gc on gc.id = g.category_id`

func (m *defaultPlatformGamesModel) ListCategories(ctx context.Context) ([]GameCategory, error) {
	query := `select id, code, name, sort_order, status from game_categories where status = 1 order by sort_order asc, id asc`
	var rows []GameCategory
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *defaultPlatformGamesModel) ListGamesPage(ctx context.Context, page, pageSize int, gameType string) ([]*PlatformGame, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "1=1"
	args := []any{}
	if gameType != "" {
		where += " AND g.game_type = ?"
		args = append(args, gameType)
	}

	countQuery := fmt.Sprintf("select count(*) from games g where %s", where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("%s where %s order by g.id desc limit ? offset ?", gameListSelect, where)
	listArgs := append(append([]any{}, args...), pageSize, offset)
	var list []*PlatformGame
	if err := m.conn.QueryRowsCtx(ctx, &list, query, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultPlatformGamesModel) FindGame(ctx context.Context, id uint64) (*PlatformGame, error) {
	query := fmt.Sprintf("%s where g.id = ? limit 1", gameListSelect)
	var row PlatformGame
	err := m.conn.QueryRowCtx(ctx, &row, query, id)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultPlatformGamesModel) FindGameByCode(ctx context.Context, code string) (*PlatformGame, error) {
	query := fmt.Sprintf("%s where g.game_code = ? limit 1", gameListSelect)
	var row PlatformGame
	err := m.conn.QueryRowCtx(ctx, &row, query, code)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultPlatformGamesModel) InsertGame(ctx context.Context, g *PlatformGame) (uint64, error) {
	query := `insert into games (game_code, name, category_id, game_type, default_rtp_ppm, volatility,
		min_bet_minor, max_bet_minor, client_version, status, launched_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var catID any
	if g.CategoryId.Valid {
		catID = g.CategoryId.Int64
	} else {
		catID = nil
	}
	var launched any
	if g.LaunchedAt.Valid {
		launched = g.LaunchedAt.Time
	} else if g.Status == 1 {
		launched = time.Now().UTC()
	}
	result, err := m.conn.ExecCtx(ctx, query,
		g.GameCode, g.Name, catID, g.GameType, g.DefaultRtpPpm, nullStringVal(g.Volatility),
		g.MinBetMinor, g.MaxBetMinor, nullStringVal(g.ClientVersion), g.Status, launched,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func (m *defaultPlatformGamesModel) UpdateGame(ctx context.Context, g *PlatformGame) error {
	query := `update games set name = ?, category_id = ?, default_rtp_ppm = ?, volatility = ?,
		min_bet_minor = ?, max_bet_minor = ?, client_version = ?, status = ? where id = ?`
	var catID any
	if g.CategoryId.Valid {
		catID = g.CategoryId.Int64
	}
	_, err := m.conn.ExecCtx(ctx, query,
		g.Name, catID, g.DefaultRtpPpm, nullStringVal(g.Volatility),
		g.MinBetMinor, g.MaxBetMinor, nullStringVal(g.ClientVersion), g.Status, g.Id,
	)
	return err
}

func (m *defaultPlatformGamesModel) ListRtpTiers(ctx context.Context, gameID uint64) ([]GameRtpTier, error) {
	query := `select id, game_id, tier_code, target_rtp_ppm, coalesce(par_sheet_ref, '') as par_sheet_ref, weight, status
		from game_rtp_tiers where game_id = ? and status = 1 order by weight desc, tier_code asc`
	var rows []GameRtpTier
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, gameID); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *defaultPlatformGamesModel) InsertRtpTier(ctx context.Context, tier *GameRtpTier) error {
	query := `insert into game_rtp_tiers (game_id, tier_code, target_rtp_ppm, par_sheet_ref, weight, status)
		values (?, ?, ?, ?, ?, 1)
		on duplicate key update target_rtp_ppm = values(target_rtp_ppm), status = 1`
	_, err := m.conn.ExecCtx(ctx, query, tier.GameId, tier.TierCode, tier.TargetRtpPpm, tier.ParSheetRef, tier.Weight)
	return err
}

const merchantGameSelect = `
select mg.id, mg.merchant_id, m.merchant_code, mg.game_id,
  g.game_code, g.name as game_name, g.game_type, g.status as game_status,
  mg.rtp_tier_code, mg.min_bet_minor, mg.max_bet_minor, mg.sort_order, mg.status, mg.opened_at,
  g.default_rtp_ppm
from merchant_games mg
join merchants m on m.id = mg.merchant_id
join games g on g.id = mg.game_id`

func (m *defaultPlatformGamesModel) ListMerchantGamesPage(ctx context.Context, merchantID uint64, page, pageSize int) ([]*MerchantGameRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		`select count(*) from merchant_games where merchant_id = ?`, merchantID); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("%s where mg.merchant_id = ? order by mg.sort_order asc, mg.id asc limit ? offset ?", merchantGameSelect)
	var list []*MerchantGameRow
	if err := m.conn.QueryRowsCtx(ctx, &list, query, merchantID, pageSize, offset); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultPlatformGamesModel) FindMerchantGame(ctx context.Context, id uint64) (*MerchantGameRow, error) {
	query := fmt.Sprintf("%s where mg.id = ? limit 1", merchantGameSelect)
	var row MerchantGameRow
	err := m.conn.QueryRowCtx(ctx, &row, query, id)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultPlatformGamesModel) MerchantGameExists(ctx context.Context, merchantID, gameID uint64) (bool, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		`select count(*) from merchant_games where merchant_id = ? and game_id = ?`, merchantID, gameID)
	return n > 0, err
}

func (m *defaultPlatformGamesModel) InsertMerchantGame(ctx context.Context, row *MerchantGameRow) (uint64, error) {
	query := `insert into merchant_games (merchant_id, game_id, rtp_tier_code, min_bet_minor, max_bet_minor, sort_order, status, opened_at)
		values (?, ?, ?, ?, ?, ?, ?, ?)`
	opened := time.Now().UTC()
	if row.Status == 0 {
		row.Status = 1
	}
	result, err := m.conn.ExecCtx(ctx, query,
		row.MerchantId, row.GameId, nullStringVal(row.RtpTierCode),
		nullInt64Val(row.MinBetMinor), nullInt64Val(row.MaxBetMinor),
		row.SortOrder, row.Status, opened,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

func (m *defaultPlatformGamesModel) UpdateMerchantGame(ctx context.Context, id uint64, rtpTierCode string, minBet, maxBet sql.NullInt64, sortOrder, status int64) error {
	query := `update merchant_games set rtp_tier_code = ?, min_bet_minor = ?, max_bet_minor = ?, sort_order = ?, status = ? where id = ?`
	_, err := m.conn.ExecCtx(ctx, query, rtpTierCode, nullInt64Val(minBet), nullInt64Val(maxBet), sortOrder, status, id)
	return err
}

func (m *defaultPlatformGamesModel) DeleteMerchantGame(ctx context.Context, id uint64) error {
	_, err := m.conn.ExecCtx(ctx, `delete from merchant_games where id = ?`, id)
	return err
}

func nullStringVal(v sql.NullString) any {
	if v.Valid {
		return v.String
	}
	return nil
}

func nullInt64Val(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
