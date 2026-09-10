package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/prng"
	"fastgame/pkg/wallet"
	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BetLogic {
	return &BetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BetLogic) Bet(req *types.BetReq) (*types.BetResp, error) {
	var resp *types.BetResp

	lockKey := fmt.Sprintf("lock:user:%d", req.UserId)
	err := l.svcCtx.Lock.WithLock(l.ctx, lockKey, 10*time.Second, func() error {
		claimed, err := l.svcCtx.Idempotent.Claim(l.ctx, req.RoundId)
		if err != nil {
			return err
		}
		if !claimed {
			return xerr.ErrDuplicateRound
		}

		betResult, err := l.svcCtx.Wallet.Bet(l.ctx, wallet.BetReq{
			MerchantID: req.MerchantId,
			UserID:     req.UserId,
			RoundID:    req.RoundId,
			Amount:     req.BetAmount,
		})
		if err != nil {
			return xerr.ErrWalletBetFailed
		}

		outcome := l.svcCtx.PRNG.Spin(req.BetAmount)
		balance := betResult.Balance

		if outcome.WinAmount > 0 {
			winResult, err := l.svcCtx.Wallet.Win(l.ctx, wallet.WinReq{
				MerchantID: req.MerchantId,
				UserID:     req.UserId,
				RoundID:    req.RoundId,
				Amount:     outcome.WinAmount,
			})
			if err != nil {
				return xerr.ErrWalletWinFailed
			}
			balance = winResult.Balance
		}

		resp = &types.BetResp{
			RoundId:      req.RoundId,
			WinAmount:    outcome.WinAmount,
			Multiplier:   outcome.Multiplier,
			Balance:      balance,
			RtpTier:      outcome.RtpTier,
			FishState:    outcome.FishState,
			AnimationKey: outcome.AnimationKey,
		}

		go l.publishSettledEvent(req, outcome, balance)
		return nil
	})
	if err != nil {
		if errors.Is(err, lock.ErrBusy) {
			return nil, xerr.ErrLockBusy
		}
		return nil, err
	}

	return resp, nil
}

func (l *BetLogic) publishSettledEvent(req *types.BetReq, outcome prng.Outcome, balance float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := l.svcCtx.Kafka.PublishRoundSettled(ctx, kafka.RoundSettledEvent{
		RoundID:    req.RoundId,
		UserID:     req.UserId,
		MerchantID: req.MerchantId,
		GameCode:   req.GameCode,
		BetAmount:  req.BetAmount,
		WinAmount:  outcome.WinAmount,
		Multiplier: outcome.Multiplier,
		RtpTier:    outcome.RtpTier,
		Balance:    balance,
	})
	if err != nil {
		logx.Errorf("publish round settled event failed: roundId=%s err=%v", req.RoundId, err)
	}
}
