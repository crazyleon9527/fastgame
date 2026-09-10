package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RtpReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRtpReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RtpReportLogic {
	return &RtpReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RtpReportLogic) RtpReport(req *types.RtpReportReq) (*types.RtpReportResp, error) {
	rows, err := l.svcCtx.Reporter.QueryRtpReport(l.ctx, req.MerchantId, req.GameCode, req.Hours)
	if err != nil || len(rows) == 0 {
		rows, err = l.svcCtx.Reporter.QueryRtpReportFromRaw(l.ctx, req.MerchantId, req.GameCode, req.Hours)
		if err != nil {
			return nil, err
		}
	}

	items := make([]types.RtpReportItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, types.RtpReportItem{
			MerchantId:  row.MerchantID,
			GameCode:    row.GameCode,
			Hour:        row.Hour.UTC().Format("2006-01-02 15:04:05"),
			TotalBet:    row.TotalBet,
			TotalWin:    row.TotalWin,
			TotalRounds: row.TotalRounds,
			ActualRtp:   row.ActualRtp,
		})
	}

	return &types.RtpReportResp{List: items}, nil
}
