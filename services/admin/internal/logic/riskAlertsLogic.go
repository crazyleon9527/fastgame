package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RiskAlertsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRiskAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RiskAlertsLogic {
	return &RiskAlertsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RiskAlertsLogic) List(req *types.RiskAlertsReq) (*types.RiskAlertsResp, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := l.svcCtx.RiskAlerts.ListOpen(l.ctx, limit)
	if err != nil {
		return nil, err
	}
	resp := &types.RiskAlertsResp{}
	for _, row := range rows {
		resp.List = append(resp.List, types.RiskAlertItem{
			Id:           row.Id,
			AlertType:    row.AlertType,
			ScopeType:    row.ScopeType,
			ScopeValue:   row.ScopeValue,
			MerchantCode: row.MerchantCode,
			GameCode:     row.GameCode,
			RtpPPM:       row.RtpPPM,
			TotalBet:     row.TotalBet,
			TotalWin:     row.TotalWin,
			SampleSize:   row.SampleSize,
			ActionTaken:  row.ActionTaken,
			Status:       row.Status,
			CreatedAt:    row.CreatedAt.Unix(),
		})
	}
	return resp, nil
}
