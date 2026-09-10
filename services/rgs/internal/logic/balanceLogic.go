package logic

import (
	"context"
	"errors"

	"fastgame/pkg/wallet"
	"fastgame/pkg/xerr"
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
		if errors.Is(err, wallet.ErrCircuitOpen) || errors.Is(err, wallet.ErrSlowResponse) {
			return nil, xerr.ErrWalletUnavailable
		}
		return nil, err
	}

	return &types.BalanceResp{Balance: balance.Minor()}, nil
}
