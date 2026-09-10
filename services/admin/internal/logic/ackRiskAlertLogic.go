package logic

import (
	"context"
	"errors"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AckRiskAlertLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAckRiskAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AckRiskAlertLogic {
	return &AckRiskAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AckRiskAlertLogic) Ack(req *types.AckRiskAlertReq) error {
	alert, err := l.svcCtx.RiskAlerts.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == model.ErrNotFound {
			return errors.New("alert not found")
		}
		return err
	}
	if err := l.svcCtx.RiskAlerts.MarkAcknowledged(l.ctx, req.Id); err != nil {
		return err
	}
	if alert.ScopeType == "user" {
		_ = l.svcCtx.Redis.Del(l.ctx, "rtp:suspend:user:"+alert.ScopeValue).Err()
	}
	if alert.ScopeType == "game" {
		key := "rtp:flag:game:" + alert.ScopeValue
		if alert.MerchantCode != "" && alert.GameCode != "" {
			key = "rtp:flag:game:" + alert.MerchantCode + ":" + alert.GameCode
		}
		_ = l.svcCtx.Redis.Del(l.ctx, key).Err()
	}
	return nil
}
