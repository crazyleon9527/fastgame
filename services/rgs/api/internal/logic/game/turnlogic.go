package game

import (
	"context"

	"fastgame/engine"
	applog "fastgame/pkg/log"
	"fastgame/pkg/money"
	"fastgame/service/rgs/api/internal/svc"
	"fastgame/service/rgs/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TurnLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTurnLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TurnLogic {
	return &TurnLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TurnLogic) Turn(req *types.TurnReq) (resp *types.TurnResp, err error) {
	// 从网关 Context 中提取玩家信息 (目前联调先取固定测试账号)
	merchantID := "M_TEST"
	merchantCode := "M_TEST"
	userID := "U1001"

	// 组装 Engine 输入参数
	in := &engine.TurnInput{
		TraceID:      l.ctx.Value("trace_id").(string),
		RoundID:      req.RoundID,
		MerchantID:   merchantID,
		MerchantCode: merchantCode,
		UserID:       userID,
		GameCode:     req.GameCode,
		Currency:     req.Currency,
		BetAmount:    money.AmountFromMinor(req.BetAmount),
		IsDemo:       false,
	}

	// 触发 UniversalHost 核心调度管道
	result, err := l.svcCtx.Host.ExecuteTurn(l.ctx, in)
	if err != nil {
		applog.C(l.ctx).Errorw("turn_execution_failed", logx.Field("err", err))
		return nil, err
	}

	return &types.TurnResp{
		RoundID:             result.RoundID,
		Balance:             result.Balance.Minor(),
		WinAmount:           result.WinAmount.Minor(),
		PayoutMultiplier:    result.PayoutMultiplier,
		PresentationPayload: result.PresentationPayload,
	}, nil
}
