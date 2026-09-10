package logic

import (
	"context"

	"fastgame/pkg/wallet"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetWalletBreakerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetWalletBreakerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetWalletBreakerLogic {
	return &ResetWalletBreakerLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ResetWalletBreakerLogic) Reset(req *types.ResetWalletBreakerReq) error {
	return wallet.ResetBreaker(l.ctx, l.svcCtx.Redis, req.MerchantCode, 0)
}
