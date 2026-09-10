package logic

import (
	"context"

	"fastgame/pkg/wallet"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWalletBreakersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListWalletBreakersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWalletBreakersLogic {
	return &ListWalletBreakersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListWalletBreakersLogic) List() (*types.WalletBreakersResp, error) {
	items, err := wallet.ListOpenBreakers(l.ctx, l.svcCtx.Redis)
	if err != nil {
		return nil, err
	}
	resp := &types.WalletBreakersResp{List: make([]types.WalletBreakerItem, 0, len(items))}
	for _, item := range items {
		resp.List = append(resp.List, types.WalletBreakerItem{
			MerchantCode: item.MerchantCode,
			Open:         item.Open,
			OpenedAt:     item.OpenedAt,
			Overridden:   item.Overridden,
		})
	}
	return resp, nil
}
