package logic

import (
	"context"
	"time"

	"fastgame/pkg/security"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotateMerchantKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRotateMerchantKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateMerchantKeyLogic {
	return &RotateMerchantKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RotateMerchantKeyLogic) RotateMerchantKey(req *types.RotateMerchantKeyReq) (*types.RotateMerchantKeyResp, error) {
	merchant, err := l.svcCtx.Merchants.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, err
	}

	graceHours := req.GracePeriodHours
	if graceHours <= 0 {
		graceHours = 24
	}
	gracePeriod := time.Duration(graceHours) * time.Hour

	newKey, err := security.GenerateSecret()
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.Merchants.RotatePrivateKey(l.ctx, req.Id, newKey, gracePeriod); err != nil {
		return nil, err
	}

	l.Infof("merchant key rotated: id=%d code=%s graceHours=%d", merchant.Id, merchant.MerchantCode, graceHours)

	return &types.RotateMerchantKeyResp{
		MerchantId:       merchant.Id,
		MerchantCode:     merchant.MerchantCode,
		NewPrivateKey:    newKey,
		GracePeriodHours: graceHours,
		RotatedAt:        time.Now().UTC().Unix(),
	}, nil
}
