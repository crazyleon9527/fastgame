package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/money"
	"fastgame/pkg/prng"
	"fastgame/pkg/trace"
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

	betAmount := money.AmountFromMinor(req.BetAmount)

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

		betStart := time.Now()
		betResult, err := l.svcCtx.Wallet.Bet(l.ctx, wallet.BetReq{
			MerchantID: req.MerchantId,
			UserID:     req.UserId,
			RoundID:    req.RoundId,
			Amount:     betAmount,
		})
		trace.Record(l.ctx, l.svcCtx.Trace, "rgs", "wallet.bet", req.RoundId,
			status(err), detailErr(err), time.Since(betStart))
		if err != nil {
			if errors.Is(err, wallet.ErrCircuitOpen) || errors.Is(err, wallet.ErrSlowResponse) {
				return xerr.ErrWalletUnavailable
			}
			return xerr.ErrWalletBetFailed
		}

		if err := l.svcCtx.PendingTx.Insert(l.ctx, &model.PendingTransaction{
			TraceID:         trace.ID(l.ctx),
			RoundID:         req.RoundId,
			MerchantCode:    req.MerchantId,
			UserID:          req.UserId,
			GameCode:        req.GameCode,
			Phase:           model.PendingTxPhaseBetDebited,
			Status:          model.PendingTxStatusPending,
			BetAmount:       betAmount.Minor(),
			ExpectedAction:  model.PendingTxActionRollbackBet,
			WalletBetStatus: "confirmed",
			WalletWinStatus: "unknown",
		}); err != nil {
			logx.Errorf("insert pending_transaction failed: roundId=%s err=%v", req.RoundId, err)
		}

		engine := prng.NewEngine(gameCfg.RtpTier)
		defer engine.Release()
		scene, proof, err := engine.ComputeReplay(
			sessionData.ServerSeed,
			sessionData.ClientSeed,
			req.RoundId,
			betAmount,
		)
		if err != nil {
			return err
		}
		outcome := scene.Outcome

		balance := betResult.Balance
		settlementStatus := "settled"

		if outcome.WinAmount > 0 {
			winStart := time.Now()
			winResult, err := l.svcCtx.Wallet.Win(l.ctx, wallet.WinReq{
				MerchantID: req.MerchantId,
				UserID:     req.UserId,
				RoundID:    req.RoundId,
				Amount:     outcome.WinAmount,
			})
			trace.Record(l.ctx, l.svcCtx.Trace, "rgs", "wallet.win", req.RoundId,
				status(err), detailErr(err), time.Since(winStart))
			if err != nil {
				opType := model.PendingOpWinFailed
				if wallet.IsTimeoutErr(err) {
					opType = model.PendingOpWinTimeout
				}
				l.recordPending(req, betAmount, outcome, opType, err)
				if err := l.svcCtx.PendingTx.MarkWinPending(l.ctx, req.RoundId, outcome.WinAmount.Minor(), err.Error()); err != nil {
					logx.Errorf("mark win pending failed: roundId=%s err=%v", req.RoundId, err)
				}
				settlementStatus = "pending"
				balance = betResult.Balance
			} else {
				balance = winResult.Balance
			}
		}

		resp = &types.BetResp{
			RoundId:          req.RoundId,
			WinAmount:        outcome.WinAmount.Minor(),
			Multiplier:       outcome.Multiplier.Minor(),
			Balance:          balance.Minor(),
			RtpTier:          outcome.RtpTier,
			FishState:        outcome.FishState,
			AnimationKey:     outcome.AnimationKey,
			SequenceId:       req.SequenceId,
			SettlementStatus: settlementStatus,
			ProvablyFair:     proofToTypes(proof),
			Replay:           SceneToPayload(scene, sessionData.ServerSeed, sessionData.ClientSeed, req.RoundId, betAmount),
		}

		if settlementStatus == "settled" {
			if err := l.svcCtx.PendingTx.MarkSettled(l.ctx, req.RoundId); err != nil {
				logx.Errorf("mark pending_transaction settled failed: roundId=%s err=%v", req.RoundId, err)
			}
			trace.Record(l.ctx, l.svcCtx.Trace, "rgs", "settlement.complete", req.RoundId, "ok", "", 0)

			if err := l.svcCtx.ReplayStore.Insert(l.ctx, &model.GameRoundReplay{
				RoundID:      req.RoundId,
				MerchantCode: req.MerchantId,
				UserID:       req.UserId,
				GameCode:     req.GameCode,
				ServerSeed:   sessionData.ServerSeed,
				ClientSeed:   sessionData.ClientSeed,
				Nonce:        req.RoundId,
				BetAmount:    betAmount.Minor(),
				SequenceID:   req.SequenceId,
			}); err != nil {
				logx.Errorf("save replay record failed: roundId=%s err=%v", req.RoundId, err)
			}
		}

		if err := l.svcCtx.Idempotent.SaveResult(l.ctx, req.RoundId, resp); err != nil {
			logx.Errorf("save idempotent result failed: roundId=%s err=%v", req.RoundId, err)
		}

		if settlementStatus == "settled" {
			go l.publishSettledEvent(req, outcome, balance)
			if outcome.Multiplier >= money.Multiplier(500000) {
				go l.publishBigWinEvent(req, outcome)
			}
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

func (l *BetLogic) recordPending(req *types.BetReq, betAmount money.Amount, outcome prng.Outcome, opType string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, insertErr := l.svcCtx.PendingOps.Insert(ctx, &model.WalletPendingOp{
		RoundID:      req.RoundId,
		MerchantCode: req.MerchantId,
		UserID:       req.UserId,
		OpType:       opType,
		BetAmount:    betAmount.Minor(),
		WinAmount:    outcome.WinAmount.Minor(),
		Status:       model.PendingStatusPending,
		LastError:    sql.NullString{String: err.Error(), Valid: err != nil},
	})
	if insertErr != nil {
		logx.Errorf("record pending op failed: roundId=%s err=%v", req.RoundId, insertErr)
	}
}

func (l *BetLogic) publishSettledEvent(req *types.BetReq, outcome prng.Outcome, balance money.Amount) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx = trace.WithID(ctx, trace.ID(l.ctx))

	err := l.svcCtx.Kafka.PublishRoundSettled(ctx, kafka.RoundSettledEvent{
		TraceID:    trace.ID(l.ctx),
		RoundID:    req.RoundId,
		UserID:     req.UserId,
		MerchantID: req.MerchantId,
		GameCode:   req.GameCode,
		BetAmount:  req.BetAmount,
		WinAmount:  outcome.WinAmount.Minor(),
		Multiplier: outcome.Multiplier.Minor(),
		RtpTier:    outcome.RtpTier,
		Balance:    balance.Minor(),
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
		WinAmount:  outcome.WinAmount.Minor(),
		Multiplier: outcome.Multiplier.Minor(),
	})
	if err != nil {
		logx.Errorf("publish bigwin event failed: roundId=%s err=%v", req.RoundId, err)
	}
}

func status(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func detailErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
