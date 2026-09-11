package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"fastgame/internal/model"
	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlatformGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformGamesLogic {
	return &PlatformGamesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PlatformGamesLogic) ListCategories() (*types.GameCategoryListResp, error) {
	rows, err := l.svcCtx.PlatformGames.ListCategories(l.ctx)
	if err != nil {
		return nil, err
	}
	items := make([]types.GameCategoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.GameCategoryItem{Id: r.Id, Code: r.Code, Name: r.Name})
	}
	return &types.GameCategoryListResp{List: items}, nil
}

func (l *PlatformGamesLogic) ListGames(req *types.PlatformGameListReq) (*types.PlatformGameListResp, error) {
	list, total, err := l.svcCtx.PlatformGames.ListGamesPage(l.ctx, req.Page, req.PageSize, req.GameType)
	if err != nil {
		return nil, err
	}
	items := make([]types.PlatformGameItem, 0, len(list))
	for _, g := range list {
		items = append(items, toPlatformGameItem(g))
	}
	return &types.PlatformGameListResp{Total: total, List: items}, nil
}

func (l *PlatformGamesLogic) CreateGame(req *types.CreatePlatformGameReq) (*types.PlatformGameItem, error) {
	if roleNameFromCtx(l.ctx) != auth.RoleAdmin {
		return nil, errors.New("only admin can create platform games")
	}
	code := strings.TrimSpace(req.GameCode)
	if code == "" {
		return nil, errors.New("gameCode required")
	}
	if _, err := l.svcCtx.PlatformGames.FindGameByCode(l.ctx, code); err == nil {
		return nil, errors.New("gameCode already exists")
	} else if err != model.ErrNotFound {
		return nil, err
	}

	rtp := req.DefaultRtpPpm
	if rtp <= 0 {
		rtp = 960000
	}
	minBet := req.MinBetMinor
	if minBet <= 0 {
		minBet = 10000
	}
	maxBet := req.MaxBetMinor
	if maxBet <= 0 {
		maxBet = 10000000
	}
	status := req.Status
	if status == 0 {
		status = 1
	}

	g := &model.PlatformGame{
		GameCode:      code,
		Name:          strings.TrimSpace(req.Name),
		GameType:      strings.TrimSpace(req.GameType),
		DefaultRtpPpm: rtp,
		MinBetMinor:   minBet,
		MaxBetMinor:   maxBet,
		Status:        status,
	}
	if req.CategoryId > 0 {
		g.CategoryId = sql.NullInt64{Int64: int64(req.CategoryId), Valid: true}
	}
	if req.Volatility != "" {
		g.Volatility = sql.NullString{String: req.Volatility, Valid: true}
	}
	if req.ClientVersion != "" {
		g.ClientVersion = sql.NullString{String: req.ClientVersion, Valid: true}
	}
	if req.ThumbnailUrl != "" {
		g.ThumbnailUrl = sql.NullString{String: req.ThumbnailUrl, Valid: true}
	}

	id, err := l.svcCtx.PlatformGames.InsertGame(l.ctx, g)
	if err != nil {
		return nil, err
	}
	_ = l.svcCtx.PlatformGames.InsertRtpTier(l.ctx, &model.GameRtpTier{
		GameId:       id,
		TierCode:     "default",
		TargetRtpPpm: rtp,
		ParSheetRef:  fmt.Sprintf("%s/par-v1", code),
		Weight:       100,
	})

	created, err := l.svcCtx.PlatformGames.FindGame(l.ctx, id)
	if err != nil {
		return nil, err
	}
	item := toPlatformGameItem(created)
	return &item, nil
}

func (l *PlatformGamesLogic) UpdateGame(req *types.UpdatePlatformGameReq) (*types.PlatformGameItem, error) {
	g, err := l.svcCtx.PlatformGames.FindGame(l.ctx, req.Id)
	if err != nil {
		return nil, errors.New("game not found")
	}
	if req.Name != "" {
		g.Name = req.Name
	}
	if req.CategoryId > 0 {
		g.CategoryId = sql.NullInt64{Int64: int64(req.CategoryId), Valid: true}
	}
	if req.DefaultRtpPpm > 0 {
		g.DefaultRtpPpm = req.DefaultRtpPpm
	}
	if req.Volatility != "" {
		g.Volatility = sql.NullString{String: req.Volatility, Valid: true}
	}
	if req.MinBetMinor > 0 {
		g.MinBetMinor = req.MinBetMinor
	}
	if req.MaxBetMinor > 0 {
		g.MaxBetMinor = req.MaxBetMinor
	}
	if req.ClientVersion != "" {
		g.ClientVersion = sql.NullString{String: req.ClientVersion, Valid: true}
	}
	if req.ThumbnailUrl != "" {
		g.ThumbnailUrl = sql.NullString{String: req.ThumbnailUrl, Valid: true}
	}
	if req.Status == 0 || req.Status == 1 || req.Status == 2 {
		g.Status = req.Status
	}
	if err := l.svcCtx.PlatformGames.UpdateGame(l.ctx, g); err != nil {
		return nil, err
	}
	updated, err := l.svcCtx.PlatformGames.FindGame(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	item := toPlatformGameItem(updated)
	return &item, nil
}

func (l *PlatformGamesLogic) ListRtpTiers(req *types.GameRtpTierListReq) (*types.GameRtpTierListResp, error) {
	rows, err := l.svcCtx.PlatformGames.ListRtpTiers(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	items := make([]types.GameRtpTierItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.GameRtpTierItem{
			Id: r.Id, GameId: r.GameId, TierCode: r.TierCode,
			TargetRtpPpm: r.TargetRtpPpm, ParSheetRef: r.ParSheetRef, Weight: r.Weight,
		})
	}
	return &types.GameRtpTierListResp{List: items}, nil
}

func toPlatformGameItem(g *model.PlatformGame) types.PlatformGameItem {
	item := types.PlatformGameItem{
		Id: g.Id, GameCode: g.GameCode, Name: g.Name,
		CategoryCode: g.CategoryCode, CategoryName: g.CategoryName,
		GameType: g.GameType, DefaultRtpPpm: g.DefaultRtpPpm,
		MinBetMinor: g.MinBetMinor, MaxBetMinor: g.MaxBetMinor, Status: g.Status,
	}
	if g.CategoryId.Valid {
		item.CategoryId = uint64(g.CategoryId.Int64)
	}
	if g.Volatility.Valid {
		item.Volatility = g.Volatility.String
	}
	if g.ClientVersion.Valid {
		item.ClientVersion = g.ClientVersion.String
	}
	if g.ThumbnailUrl.Valid {
		item.ThumbnailUrl = g.ThumbnailUrl.String
	}
	return item
}
