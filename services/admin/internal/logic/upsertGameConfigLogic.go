package logic

import (
	"context"
	"database/sql"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertGameConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpsertGameConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertGameConfigLogic {
	return &UpsertGameConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertGameConfigLogic) UpsertGameConfig(req *types.UpsertGameConfigReq) (*types.GameConfigItem, error) {
	status := req.Status
	if status == 0 {
		status = 1
	}

	existing, err := l.svcCtx.GameConfigs.FindOneByMerchantIdGameCodeConfigKey(
		l.ctx, req.MerchantId, req.GameCode, req.ConfigKey,
	)
	if err == nil {
		existing.ConfigValue = req.ConfigValue
		existing.Status = status
		if req.RtpTier != "" {
			existing.RtpTier = sql.NullString{String: req.RtpTier, Valid: true}
		}
		if err := l.svcCtx.GameConfigs.Update(l.ctx, existing); err != nil {
			return nil, err
		}
		return toGameConfigItem(existing), nil
	}

	result, err := l.svcCtx.GameConfigs.Insert(l.ctx, &model.GameConfigs{
		MerchantId:  req.MerchantId,
		GameCode:    req.GameCode,
		ConfigKey:   req.ConfigKey,
		ConfigValue: req.ConfigValue,
		RtpTier:     sql.NullString{String: req.RtpTier, Valid: req.RtpTier != ""},
		Status:      status,
	})
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &types.GameConfigItem{
		Id:          uint64(id),
		MerchantId:  req.MerchantId,
		GameCode:    req.GameCode,
		ConfigKey:   req.ConfigKey,
		ConfigValue: req.ConfigValue,
		RtpTier:     req.RtpTier,
		Status:      status,
	}, nil
}

func toGameConfigItem(cfg *model.GameConfigs) *types.GameConfigItem {
	rtpTier := ""
	if cfg.RtpTier.Valid {
		rtpTier = cfg.RtpTier.String
	}
	return &types.GameConfigItem{
		Id:          cfg.Id,
		MerchantId:  cfg.MerchantId,
		GameCode:    cfg.GameCode,
		ConfigKey:   cfg.ConfigKey,
		ConfigValue: cfg.ConfigValue,
		RtpTier:     rtpTier,
		Status:      cfg.Status,
	}
}
