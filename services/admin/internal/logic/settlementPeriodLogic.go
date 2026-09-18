package logic

import (
	"context"
	"database/sql"
	"errors"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SettlementPeriodLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettlementPeriodLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlementPeriodLogic {
	return &SettlementPeriodLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SettlementPeriodLogic) List(req *types.SettlementPeriodListReq) (*types.SettlementPeriodListResp, error) {
	list, total, err := l.svcCtx.SettlementPeriods.ListPage(l.ctx, req.MerchantId, req.PeriodType, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	merchantNames := map[uint64]string{}
	if merchants, _, mErr := l.svcCtx.Merchants.ListPage(l.ctx, 1, 500); mErr == nil {
		for _, m := range merchants {
			merchantNames[m.Id] = m.MerchantCode
		}
	}

	items := make([]types.SettlementPeriodItem, 0, len(list))
	for _, row := range list {
		items = append(items, types.SettlementPeriodItem{
			Id:           row.Id,
			MerchantId:   row.MerchantId,
			MerchantCode: merchantNames[row.MerchantId],
			PeriodType:   row.PeriodType,
			PeriodStart:  row.PeriodStart.Format("2006-01-02"),
			PeriodEnd:    row.PeriodEnd.Format("2006-01-02"),
			CurrencyCode: row.CurrencyCode,
			TotalBet:     minorToMajor(row.TotalBetMinor),
			TotalWin:     minorToMajor(row.TotalWinMinor),
			Ggr:          minorToMajor(row.GgrMinor),
			Commission:   minorToMajor(row.CommissionMinor),
			NetPayable:   minorToMajor(row.NetPayableMinor),
			TotalRounds:  row.TotalRounds,
			Status:       row.Status,
			ConfirmedBy:  uint64(row.ConfirmedBy.Int64),
			ConfirmedAt:  nullTimeToStr(row.ConfirmedAt),
			Notes:        row.Notes.String,
		})
	}
	return &types.SettlementPeriodListResp{Total: total, List: items}, nil
}

// Rollup re-generates the settlement period from confirmed daily settlements.
// Idempotent: re-running refreshes totals and backfills daily links.
func (l *SettlementPeriodLogic) Rollup(req *types.RollupSettlementPeriodReq) (*types.RollupSettlementPeriodResp, error) {
	period, err := l.svcCtx.SettlementPeriods.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, errors.New("settlement period not found")
	}
	if period.Status != "draft" {
		return nil, errors.New("only draft periods can be rolled up")
	}

	pid, err := l.svcCtx.SettlementPeriods.GenerateFromDaily(
		l.ctx, period.MerchantId, period.PeriodType, period.PeriodStart, period.PeriodEnd, period.CurrencyCode,
	)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.SettlementPeriods.SetPeriodOnDaily(
		l.ctx, period.MerchantId, uint64(pid), period.PeriodStart, period.PeriodEnd,
	); err != nil {
		return nil, err
	}
	return &types.RollupSettlementPeriodResp{RolledUp: 1}, nil
}

func nullTimeToStr(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02 15:04:05")
}
