package logic

import (
	"context"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMerchantAllowedIPsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMerchantAllowedIPsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMerchantAllowedIPsLogic {
	return &GetMerchantAllowedIPsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMerchantAllowedIPsLogic) GetMerchantAllowedIPs(req *types.MerchantAllowedIPsReq) (*types.MerchantAllowedIPsResp, error) {
	merchantCode, ips, err := l.svcCtx.Merchants.FindAllowedIPsByID(l.ctx, req.Id)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, err
		}
		return nil, err
	}
	if ips == nil {
		ips = []string{}
	}
	return &types.MerchantAllowedIPsResp{
		MerchantId:   req.Id,
		MerchantCode: merchantCode,
		AllowedIps:   ips,
	}, nil
}
