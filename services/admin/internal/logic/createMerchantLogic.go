package logic

import (
	"context"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateMerchantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateMerchantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMerchantLogic {
	return &CreateMerchantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateMerchantLogic) CreateMerchant(req *types.CreateMerchantReq) (*types.MerchantItem, error) {
	status := req.Status
	if status == 0 {
		status = 1
	}

	result, err := l.svcCtx.Merchants.Insert(l.ctx, &model.Merchants{
		MerchantCode: req.MerchantCode,
		Name:         req.Name,
		Status:       status,
	})
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &types.MerchantItem{
		Id:           uint64(id),
		MerchantCode: req.MerchantCode,
		Name:         req.Name,
		Status:       status,
	}, nil
}
