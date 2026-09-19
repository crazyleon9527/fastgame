package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	applog "fastgame/pkg/log"
	"fastgame/pkg/money"
	"fastgame/pkg/outbox"
	"fastgame/pkg/prng"
	"fastgame/pkg/session"
	"fastgame/pkg/trace"
	"fastgame/pkg/wallet"
	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type BetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BetLogic {
	return &BetLogic{
		Logger: applog.C(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BetLogic) Bet(req *types.BetReq) (*types.BetResp, error) {
	l.ctx = applog.WithRoundContext(l.ctx, applog.RoundFields{
		MerchantCode: req.MerchantId,
		GameCode:     req.GameCode,
		UserId:       req.UserId,
		RoundId:      req.RoundId,
	})
	l.Logger = applog.C(l.ctx)

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
	lockKey := lock.BetLockKey(req.MerchantId, req.UserId)
	err := l.svcCtx.Lock.WithLock(l.ctx, lockKey, lock.DefaultBetLockTTL, func() error {
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
			MerchantID:      gameCfg.MerchantID,
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
				l.recordPending(req, betAmount, outcome, opType, err, gameCfg.MerchantID)
				if err := l.svcCtx.PendingTx.MarkWinPending(l.ctx, gameCfg.MerchantID, req.RoundId, outcome.WinAmount.Minor(), err.Error()); err != nil {
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
			// 结算收尾：账变已由钱包完成，这里把「本地对账标记 + 回放记录 + 事件登记」
			// 收进同一个事务。事务提交成功即保证事件不会丢（由 outbox 负责投递），
			// 取代原先的裸 `go publishSettledEvent(...)`（投递失败无人知晓）。
			if err := l.recordSettled(req, sessionData, outcome, betAmount, balance, gameCfg.MerchantID); err != nil {
				// 不阻断下注返回：钱已在钱包侧结算完成，此处失败由 pending_transactions
				// （phase=bet_debited）与 rollback 服务的对账兜住。
				logx.Errorf("record settled bookkeeping failed: roundId=%s err=%v", req.RoundId, err)
			}
		}

		if err := l.svcCtx.Idempotent.SaveResult(l.ctx, req.RoundId, resp); err != nil {
			logx.Errorf("save idempotent result failed: roundId=%s err=%v", req.RoundId, err)
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

func (l *BetLogic) recordPending(req *types.BetReq, betAmount money.Amount, outcome prng.Outcome, opType string, insertionErr error, merchantID uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, insertErr := l.svcCtx.PendingOps.Insert(ctx, &model.WalletPendingOp{
		RoundID:      req.RoundId,
		MerchantID:   merchantID,
		MerchantCode: req.MerchantId,
		UserID:       req.UserId,
		OpType:       opType,
		BetAmount:    betAmount.Minor(),
		WinAmount:    outcome.WinAmount.Minor(),
		Status:       model.PendingStatusPending,
		LastError:    sql.NullString{String: insertionErr.Error(), Valid: insertionErr != nil},
	})
	if insertErr != nil {
		logx.Errorf("record pending op failed: roundId=%s err=%v", req.RoundId, insertErr)
	}
}

// recordSettled 在一个事务里完成结算收尾：
//
//	MarkSettled（本地对账标记）+ 回放记录 + 结算事件登记（outbox）
//
// 三者同事务提交，因此「事件一定不会丢」这件事由数据库保证，而不是靠
// goroutine 是否跑成功。投递交给 outbox.Dispatcher（失败会退避重试）。
//
// 旁路的 trace 埋点与幂等结果缓存不进事务：前者是观测数据，
// 后者是 Redis，都不应与资金相关的事务成败互相牵连。
func (l *BetLogic) recordSettled(
	req *types.BetReq,
	sessionData *session.Data,
	outcome prng.Outcome,
	betAmount money.Amount,
	balance money.Amount,
	merchantID uint64,
) error {
	trace.Record(l.ctx, l.svcCtx.Trace, "rgs", "settlement.complete", req.RoundId, "ok", "", 0)

	return l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		txConn := sqlx.NewSqlConnFromSession(session)

		// 1) 本地对账标记：该局已结算，rollback 服务不会再当孤儿单处理
		//
		// 注意：这两张表的唯一键都是 (merchant_id, round_id)，所以 merchant_id
		// 必须如实写入。若留 0（列默认值），多商户下不同商户的同一 roundId 会
		// 落到同一个 (0, roundId) 上互相覆盖，唯一键形同虚设。
		if err := model.NewPendingTransactionsModel(txConn).MarkSettled(ctx, merchantID, req.RoundId); err != nil {
			return fmt.Errorf("mark pending_transaction settled: %w", err)
		}

		// 2) 确定性回放记录（供 /verify 与 /replay 使用）
		if err := model.NewGameRoundReplayModel(txConn).Insert(ctx, &model.GameRoundReplay{
			RoundID:      req.RoundId,
			MerchantID:   merchantID,
			MerchantCode: req.MerchantId,
			UserID:       req.UserId,
			GameCode:     req.GameCode,
			ServerSeed:   sessionData.ServerSeed,
			ClientSeed:   sessionData.ClientSeed,
			Nonce:        req.RoundId,
			BetAmount:    betAmount.Minor(),
			SequenceID:   req.SequenceId,
		}); err != nil {
			return fmt.Errorf("save replay record: %w", err)
		}

		// 3) 结算事件登记（outbox，与上面两步同事务）
		//
		// 注意构造顺序：必须先生成 eventID 并写入 payload，再序列化信封。
		// 反过来（先序列化、后赋值）会导致内层 payload.eventId 为空，
		// 消费端就无法用它与信封 eventId 对账。
		settledID, err := outbox.NewEventID()
		if err != nil {
			return fmt.Errorf("generate settled event id: %w", err)
		}
		settled := kafka.RoundSettledEvent{
			EventID:    settledID,
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
			SettledAt:  time.Now().UTC(),
		}
		env, err := outbox.NewEnvelopeWithID(
			outbox.EventTopic(kafka.TopicRoundSettled), "rgs-api", trace.ID(l.ctx), settledID, settled, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("build settled envelope: %w", err)
		}
		if err := l.svcCtx.Outbox.InsertInTx(ctx, session, &outbox.Record{
			ID:    settledID,
			Topic: kafka.TopicRoundSettled,
			// merchant_id 与 merchant_code 都要落：前者是入库口径（merchants.id），
			// 后者是信令口径。event_outbox 不设 merchant_id 就查不出"某商户的待投递事件"。
			MerchantID:   merchantID,
			PartitionKey: req.UserId, // 同玩家进同一分区，保证时序
			Payload:      env,
		}); err != nil {
			return fmt.Errorf("outbox insert settled: %w", err)
		}

		// 4) 大奖广播单独一个 topic（倍率 >= 50x）
		if outcome.Multiplier >= money.Multiplier(500000) {
			bigwinID, err := outbox.NewEventID()
			if err != nil {
				return fmt.Errorf("generate bigwin event id: %w", err)
			}
			bigwin := kafka.BigWinEvent{
				EventID:    bigwinID,
				TraceID:    trace.ID(l.ctx),
				RoundID:    req.RoundId,
				UserID:     req.UserId,
				MerchantID: req.MerchantId,
				GameCode:   req.GameCode,
				WinAmount:  outcome.WinAmount.Minor(),
				Multiplier: outcome.Multiplier.Minor(),
				OccurredAt: time.Now().UTC(),
			}
			bwEnv, err := outbox.NewEnvelopeWithID(
				outbox.EventTopic(kafka.TopicEventBigwin), "rgs-api", trace.ID(l.ctx), bigwinID, bigwin, time.Now().UTC())
			if err != nil {
				return fmt.Errorf("build bigwin envelope: %w", err)
			}
			if err := l.svcCtx.Outbox.InsertInTx(ctx, session, &outbox.Record{
				ID:           bigwinID,
				Topic:        kafka.TopicEventBigwin,
				MerchantID:   merchantID,
				PartitionKey: req.UserId,
				Payload:      bwEnv,
			}); err != nil {
				return fmt.Errorf("outbox insert bigwin: %w", err)
			}
		}
		return nil
	})
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
