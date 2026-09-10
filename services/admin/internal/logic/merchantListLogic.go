package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MerchantListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMerchantListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MerchantListLogic {
	return &MerchantListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MerchantListLogic) MerchantList(req *types.MerchantListReq) (*types.MerchantListResp, error) {
	list, total, err := l.svcCtx.Merchants.ListPage(l.ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	items := make([]types.MerchantItem, 0, len(list))
	for _, m := range list {
		items = append(items, types.MerchantItem{
			Id:           m.Id,
			MerchantCode: m.MerchantCode,
			Name:         m.Name,
			Status:       m.Status,
		})
	}

	return &types.MerchantListResp{Total: total, List: items}, nil
}
