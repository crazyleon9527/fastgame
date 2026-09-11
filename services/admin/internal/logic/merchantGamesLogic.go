package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MerchantGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMerchantGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MerchantGamesLogic {
	return &MerchantGamesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MerchantGamesLogic) List(req *types.MerchantGameListReq) (*types.MerchantGameListResp, error) {
	list, total, err := l.svcCtx.PlatformGames.ListMerchantGamesPage(l.ctx, req.MerchantId, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]types.MerchantGameItem, 0, len(list))
	for _, row := range list {
		items = append(items, toMerchantGameItem(row))
	}
	return &types.MerchantGameListResp{Total: total, List: items}, nil
}

func (l *MerchantGamesLogic) Create(req *types.CreateMerchantGameReq) (*types.MerchantGameItem, error) {
	if req.MerchantId == 0 || req.GameId == 0 {
		return nil, errors.New("merchantId and gameId required")
	}
	if _, err := l.svcCtx.Merchants.FindOne(l.ctx, req.MerchantId); err != nil {
		return nil, errors.New("merchant not found")
	}
	if _, err := l.svcCtx.PlatformGames.FindGame(l.ctx, req.GameId); err != nil {
		return nil, errors.New("game not found")
	}
	exists, err := l.svcCtx.PlatformGames.MerchantGameExists(l.ctx, req.MerchantId, req.GameId)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("game already opened for this merchant")
	}

	tier := strings.TrimSpace(req.RtpTierCode)
	if tier == "" {
		tier = "default"
	}
	row := &model.MerchantGameRow{
		MerchantId: req.MerchantId,
		GameId:     req.GameId,
		RtpTierCode: sql.NullString{String: tier, Valid: true},
		SortOrder:  req.SortOrder,
		Status:     1,
	}
	if req.MinBetMinor > 0 {
		row.MinBetMinor = sql.NullInt64{Int64: req.MinBetMinor, Valid: true}
	}
	if req.MaxBetMinor > 0 {
		row.MaxBetMinor = sql.NullInt64{Int64: req.MaxBetMinor, Valid: true}
	}

	id, err := l.svcCtx.PlatformGames.InsertMerchantGame(l.ctx, row)
	if err != nil {
		return nil, err
	}
	created, err := l.svcCtx.PlatformGames.FindMerchantGame(l.ctx, id)
	if err != nil {
		return nil, err
	}
	item := toMerchantGameItem(created)
	return &item, nil
}

func (l *MerchantGamesLogic) Update(req *types.UpdateMerchantGameReq) (*types.MerchantGameItem, error) {
	current, err := l.svcCtx.PlatformGames.FindMerchantGame(l.ctx, req.Id)
	if err != nil {
		return nil, errors.New("merchant game not found")
	}

	tier := current.RtpTierCode.String
	if req.RtpTierCode != "" {
		tier = req.RtpTierCode
	}
	minBet := current.MinBetMinor
	if req.MinBetMinor > 0 {
		minBet = sql.NullInt64{Int64: req.MinBetMinor, Valid: true}
	}
	maxBet := current.MaxBetMinor
	if req.MaxBetMinor > 0 {
		maxBet = sql.NullInt64{Int64: req.MaxBetMinor, Valid: true}
	}
	sortOrder := current.SortOrder
	if req.SortOrder > 0 {
		sortOrder = req.SortOrder
	}
	status := current.Status
	if req.Status == 0 || req.Status == 1 {
		status = req.Status
	}

	if err := l.svcCtx.PlatformGames.UpdateMerchantGame(l.ctx, req.Id, tier, minBet, maxBet, sortOrder, status); err != nil {
		return nil, err
	}
	updated, err := l.svcCtx.PlatformGames.FindMerchantGame(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}
	item := toMerchantGameItem(updated)
	return &item, nil
}

func (l *MerchantGamesLogic) Delete(req *types.DeleteMerchantGameReq) error {
	if _, err := l.svcCtx.PlatformGames.FindMerchantGame(l.ctx, req.Id); err != nil {
		return errors.New("merchant game not found")
	}
	return l.svcCtx.PlatformGames.DeleteMerchantGame(l.ctx, req.Id)
}

func toMerchantGameItem(row *model.MerchantGameRow) types.MerchantGameItem {
	item := types.MerchantGameItem{
		Id: row.Id, MerchantId: row.MerchantId, MerchantCode: row.MerchantCode,
		GameId: row.GameId, GameCode: row.GameCode, GameName: row.GameName,
		GameType: row.GameType, GameStatus: row.GameStatus,
		SortOrder: row.SortOrder, Status: row.Status, DefaultRtpPpm: row.DefaultRtpPpm,
	}
	if row.RtpTierCode.Valid {
		item.RtpTierCode = row.RtpTierCode.String
	}
	if row.MinBetMinor.Valid {
		item.MinBetMinor = row.MinBetMinor.Int64
	}
	if row.MaxBetMinor.Valid {
		item.MaxBetMinor = row.MaxBetMinor.Int64
	}
	return item
}
