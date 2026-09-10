package logic

import (
	"context"

	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BalanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BalanceLogic {
	return &BalanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BalanceLogic) Balance(req *types.BalanceReq) (*types.BalanceResp, error) {
	balance, err := l.svcCtx.Wallet.GetBalance(l.ctx, req.MerchantId, req.UserId)
	if err != nil {
		return nil, err
	}

	return &types.BalanceResp{Balance: balance}, nil
}
