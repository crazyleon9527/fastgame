package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMerchantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMerchantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMerchantLogic {
	return &UpdateMerchantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMerchantLogic) UpdateMerchant(req *types.UpdateMerchantReq) (*types.MerchantItem, error) {
	merchant, err := l.svcCtx.Merchants.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		merchant.Name = req.Name
	}
	if req.Status != 0 {
		merchant.Status = req.Status
	}

	if err := l.svcCtx.Merchants.Update(l.ctx, merchant); err != nil {
		return nil, err
	}

	return &types.MerchantItem{
		Id:           merchant.Id,
		MerchantCode: merchant.MerchantCode,
		Name:         merchant.Name,
		Status:       merchant.Status,
	}, nil
}
