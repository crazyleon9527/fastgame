package logic

import (
	"context"
	"errors"
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
	if req.Action != "cast" {
		return nil, xerr.ErrInvalidRequest
	}

	idempotencyToken := req.IdempotencyToken
	if idempotencyToken == "" {
		idempotencyToken = req.RoundId
	}

	var cached types.BetResp
	if ok, err := l.svcCtx.Idempotent.GetResult(l.ctx, req.RoundId, &cached); err != nil {
		return nil, err
	} else if ok {
		return &cached, nil
	}

	var resp *types.BetResp
	err := l.svcCtx.Lock.WithLock(l.ctx, lock.BetLockKey(req.UserId), lock.BetLockTTL, func() error {
		if ok, err := l.svcCtx.Idempotent.GetResult(l.ctx, req.RoundId, &cached); err != nil {
			return err
		} else if ok {
			resp = &cached
			return nil
		}

		sessionData, err := l.svcCtx.Session.ConsumeSequence(
			l.ctx, req.SessionToken, req.UserId, req.MerchantId, req.GameCode, req.SequenceId,
		)
		if err != nil {
			if err.Error() == "invalid sequence id" {
				return xerr.ErrInvalidSequence
			}
			return xerr.ErrInvalidSession
		}
		if req.ClientSeed != "" && req.ClientSeed != sessionData.ClientSeed {
			return xerr.ErrInvalidRequest
		}

		claimed, err := l.svcCtx.Idempotent.ClaimWithToken(l.ctx, req.RoundId, idempotencyToken)
		if err != nil {
			return xerr.ErrInvalidRequest
		}
		if !claimed {
			if ok, err := l.svcCtx.Idempotent.GetResult(l.ctx, req.RoundId, &cached); err != nil {
				return err
			} else if ok {
				resp = &cached
				return nil
			}
			return xerr.ErrDuplicateRound
		}

		gameCfg, err := l.svcCtx.GameConfig.Load(l.ctx, req.MerchantId, req.GameCode)
		if err != nil {
			return err
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

		outcome, proof, err := prng.NewEngine(gameCfg.RtpTier).Spin(
			sessionData.ServerSeed,
			sessionData.ClientSeed,
			req.RoundId,
			req.BetAmount,
		)
		if err != nil {
			return err
		}

		balance := betResult.Balance
		if outcome.WinAmount > 0 {
			winResult, err := l.svcCtx.Wallet.Win(l.ctx, wallet.WinReq{
				MerchantID: req.MerchantId,
				UserID:     req.UserId,
				RoundID:    req.RoundId,
				Amount:     outcome.WinAmount,
			})
			if err != nil {
				l.handleWinFailure(req, req.BetAmount)
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
			SequenceId:   req.SequenceId,
			ProvablyFair: types.ProvablyFairProof{
				ServerSeedHash: proof.ServerSeedHash,
				ServerSeed:     proof.ServerSeed,
				ClientSeed:     proof.ClientSeed,
				Nonce:          proof.Nonce,
				Roll:           proof.Roll,
			},
		}

		if err := l.svcCtx.Idempotent.SaveResult(l.ctx, req.RoundId, resp); err != nil {
			logx.Errorf("save idempotent result failed: roundId=%s err=%v", req.RoundId, err)
		}

		go l.publishSettledEvent(req, outcome, balance)
		if outcome.Multiplier >= 50 {
			go l.publishBigWinEvent(req, outcome)
		}
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

func (l *BetLogic) handleWinFailure(req *types.BetReq, betAmount float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rollbackReq := wallet.RollbackReq{
		MerchantID: req.MerchantId,
		UserID:     req.UserId,
		RoundID:    req.RoundId,
		Amount:     betAmount,
		Reason:     "win_failed",
	}
	if err := l.svcCtx.Wallet.Rollback(ctx, rollbackReq); err != nil {
		logx.Errorf("wallet rollback failed: roundId=%s err=%v", req.RoundId, err)
	}

	go l.publishRollbackEvent(req, betAmount, "win_failed")
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

func (l *BetLogic) publishBigWinEvent(req *types.BetReq, outcome prng.Outcome) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := l.svcCtx.Kafka.PublishBigWin(ctx, kafka.BigWinEvent{
		RoundID:    req.RoundId,
		UserID:     req.UserId,
		MerchantID: req.MerchantId,
		GameCode:   req.GameCode,
		WinAmount:  outcome.WinAmount,
		Multiplier: outcome.Multiplier,
	})
	if err != nil {
		logx.Errorf("publish bigwin event failed: roundId=%s err=%v", req.RoundId, err)
	}
}

func (l *BetLogic) publishRollbackEvent(req *types.BetReq, amount float64, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := l.svcCtx.Kafka.PublishWalletRollback(ctx, kafka.WalletRollbackEvent{
		RoundID:      req.RoundId,
		UserID:       req.UserId,
		MerchantID:   req.MerchantId,
		RollbackType: "bet_refund",
		Amount:       amount,
		Reason:       reason,
	})
	if err != nil {
		logx.Errorf("publish wallet rollback event failed: roundId=%s err=%v", req.RoundId, err)
	}
}
