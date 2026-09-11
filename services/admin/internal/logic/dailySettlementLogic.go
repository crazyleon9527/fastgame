package logic

import (
	"context"
	"errors"

	"fastgame/pkg/money"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DailySettlementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDailySettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DailySettlementLogic {
	return &DailySettlementLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DailySettlementLogic) List(req *types.DailySettlementListReq) (*types.DailySettlementListResp, error) {
	list, total, err := l.svcCtx.DailySettlements.ListPage(l.ctx, req.MerchantId, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	merchantNames := map[uint64]string{}
	if merchants, _, mErr := l.svcCtx.Merchants.ListPage(l.ctx, 1, 500); mErr == nil {
		for _, m := range merchants {
			merchantNames[m.Id] = m.MerchantCode
		}
	}

	items := make([]types.DailySettlementItem, 0, len(list))
	for _, row := range list {
		items = append(items, types.DailySettlementItem{
			Id:           row.Id,
			MerchantId:   row.MerchantId,
			MerchantCode: merchantNames[row.MerchantId],
			SettleDate:   row.SettleDate.Format("2006-01-02"),
			TotalBet:     minorToMajor(row.TotalBet),
			TotalWin:     minorToMajor(row.TotalWin),
			TotalRounds:  row.TotalRounds,
			ActualRtp:    calcRtp(row.TotalBet, row.TotalWin),
			Status:       row.Status,
		})
	}
	return &types.DailySettlementListResp{Total: total, List: items}, nil
}

func (l *DailySettlementLogic) Sync(req *types.SyncDailySettlementReq) (*types.SyncDailySettlementResp, error) {
	days := req.Days
	if days <= 0 {
		days = 7
	}
	rows, err := l.svcCtx.Reporter.QueryDailySettlement(l.ctx, req.MerchantId, days)
	if err != nil {
		return nil, err
	}

	synced := 0
	for _, row := range rows {
		if err := l.svcCtx.DailySettlements.UpsertPending(
			l.ctx, row.MerchantID, row.SettleDate, row.TotalBet, row.TotalWin, row.TotalRounds,
		); err != nil {
			return nil, err
		}
		synced++
	}
	return &types.SyncDailySettlementResp{Synced: synced}, nil
}

func (l *DailySettlementLogic) Confirm(req *types.ConfirmDailySettlementReq) error {
	if err := l.svcCtx.DailySettlements.Confirm(l.ctx, req.Id); err != nil {
		return errors.New("settlement not found or already confirmed")
	}
	return nil
}

func minorToMajor(v int64) float64 {
	return float64(v) / float64(money.Scale)
}

func calcRtp(totalBet, totalWin int64) float64 {
	if totalBet <= 0 {
		return 0
	}
	return float64(totalWin) / float64(totalBet)
}
