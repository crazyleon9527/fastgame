package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameConfigListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameConfigListLogic {
	return &GameConfigListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameConfigListLogic) GameConfigList(req *types.GameConfigListReq) (*types.GameConfigListResp, error) {
	list, err := l.svcCtx.GameConfigs.FindByMerchant(l.ctx, req.MerchantId, req.GameCode)
	if err != nil {
		return nil, err
	}

	items := make([]types.GameConfigItem, 0, len(list))
	for _, cfg := range list {
		rtpTier := ""
		if cfg.RtpTier.Valid {
			rtpTier = cfg.RtpTier.String
		}
		items = append(items, types.GameConfigItem{
			Id:          cfg.Id,
			MerchantId:  cfg.MerchantId,
			GameCode:    cfg.GameCode,
			ConfigKey:   cfg.ConfigKey,
			ConfigValue: cfg.ConfigValue,
			RtpTier:     rtpTier,
			Status:      cfg.Status,
		})
	}

	return &types.GameConfigListResp{List: items}, nil
}
