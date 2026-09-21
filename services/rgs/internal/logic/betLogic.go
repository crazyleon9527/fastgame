package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/ledger"
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
	if betAmount <= 0 {
		return nil, xerr.ErrInvalidRequest
	}

	idempotencyToken := req.IdempotencyToken
	if idempotencyToken == "" {
		idempotencyToken = req.RoundId
	}

	// 1. 优先查缓存（带商户隔离）
	var cached types.BetResp
	if ok, err := l.svcCtx.Idempotent.GetResult(l.ctx, req.MerchantId, req.RoundId, &cached); err != nil {
		return nil, err
	} else if ok {
		return &cached, nil
	}

	var resp *types.BetResp
	lockKey := lock.BetLockKey(req.MerchantId, req.UserId)
	err := l.svcCtx.Lock.WithLock(l.ctx, lockKey, lock.DefaultBetLockTTL, func() error {
		// 双重检查防重
		if ok, err := l.svcCtx.Idempotent.GetResult(l.ctx, req.MerchantId, req.RoundId, &cached); err != nil {
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

		// 原子占用幂等锁（支持商户租户隔离与 Token 冲突校验）
		claimed, err := l.svcCtx.Idempotent.ClaimWithToken(l.ctx, req.MerchantId, req.RoundId, idempotencyToken)
		if err != nil {
			if errors.Is(err, idempotent.ErrTokenReused) {
				return xerr.ErrInvalidRequest
			}
			return err
		}
		if !claimed {
			// 若并发重试请求稍微落后，自旋等待 500ms 尝试直接读取第一笔的处理结果并返回
			if ok, err := l.svcCtx.Idempotent.WaitAndGetResult(l.ctx, req.MerchantId, req.RoundId, &cached, 500*time.Millisecond); err == nil && ok {
				resp = &cached
				return nil
			}
			return xerr.ErrDuplicateRound
		}

		gameCfg, err := l.svcCtx.GameConfig.Load(l.ctx, req.MerchantId, req.GameCode)
		if err != nil {
			_ = l.svcCtx.Idempotent.ReleaseClaim(l.ctx, req.MerchantId, req.RoundId)
			return err
		}

		// 2. 调用钱包下注扣款
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
			// 扣款未成功，释放幂等锁定允许重试
			_ = l.svcCtx.Idempotent.ReleaseClaim(l.ctx, req.MerchantId, req.RoundId)
			if errors.Is(err, wallet.ErrCircuitOpen) || errors.Is(err, wallet.ErrSlowResponse) {
				return xerr.ErrWalletUnavailable.WithCause(err)
			}
			return xerr.ErrWalletBetFailed.WithCause(err)
		}

		// 钱包已确认扣款，记录本地下注影子账
		l.postBet(req, gameCfg.MerchantID, betAmount, betResult.Balance)

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
			logx.WithContext(l.ctx).Errorf("insert pending_transaction failed: roundId=%s err=%v", req.RoundId, err)
		}

		// 3. 确定性 PRNG 推演
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

		// 4. 派彩逻辑
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
					logx.WithContext(l.ctx).Errorf("mark win pending failed: roundId=%s err=%v", req.RoundId, err)
				}
				settlementStatus = "pending"
				balance = betResult.Balance
				l.postLedger(req, gameCfg.MerchantID, model.TxTypeWin, outcome.WinAmount,
					nil, model.LedgerStatusPendingRetry, "派彩失败待补偿", map[string]any{
						"op_type":    opType,
						"last_error": err.Error(),
					})
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

		// 5. 终态入库与 Outbox 发件箱登记（同事务）
		if settlementStatus == "settled" {
			if err := l.recordSettled(req, sessionData, outcome, betAmount, balance, gameCfg.MerchantID); err != nil {
				logx.WithContext(l.ctx).Errorf("record settled bookkeeping failed: roundId=%s err=%v", req.RoundId, err)
			}
		}

		// 6. 保存幂等终态
		if err := l.svcCtx.Idempotent.SaveResult(l.ctx, req.MerchantId, req.RoundId, resp); err != nil {
			logx.WithContext(l.ctx).Errorf("save idempotent result failed: roundId=%s err=%v", req.RoundId, err)
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
		logx.WithContext(l.ctx).Errorf("record pending op failed: roundId=%s err=%v", req.RoundId, insertErr)
	}
}

func (l *BetLogic) postBet(req *types.BetReq, merchantID uint64, betAmount money.Amount, walletBalance money.Amount) {
	res, err := l.svcCtx.Ledger.Post(l.ctx, ledger.Posting{
		TypeCode:      model.TxTypeBet,
		MerchantID:    merchantID,
		MerchantCode:  req.MerchantId,
		UserID:        req.UserId,
		GameCode:      req.GameCode,
		RoundID:       req.RoundId,
		Amount:        betAmount,
		WalletBalance: &walletBalance,
		Status:        model.LedgerStatusSuccess,
		RefType:       "game_round",
		RefID:         req.RoundId,
		Remark:        "下注扣款",
	})
	if err != nil {
		logx.WithContext(l.ctx).Errorf("post bet ledger failed: roundId=%s err=%v", req.RoundId, err)
		return
	}
	if res.Drift.Minor() != 0 {
		logx.WithContext(l.ctx).Errorf("bet ledger drift: roundId=%s drift=%d（本地推算与钱包余额不一致，需核查）",
			req.RoundId, res.Drift.Minor())
	}
}

func (l *BetLogic) postLedger(
	req *types.BetReq,
	merchantID uint64,
	typeCode string,
	amount money.Amount,
	walletBalance *money.Amount,
	status, remark string,
	extra map[string]any,
) {
	if amount <= 0 {
		return
	}
	if _, err := l.svcCtx.Ledger.Post(l.ctx, ledger.Posting{
		TypeCode:      typeCode,
		MerchantID:    merchantID,
		MerchantCode:  req.MerchantId,
		UserID:        req.UserId,
		GameCode:      req.GameCode,
		RoundID:       req.RoundId,
		Amount:        amount,
		WalletBalance: walletBalance,
		Status:        status,
		RefType:       "game_round",
		RefID:         req.RoundId,
		Remark:        remark,
		Extra:         extra,
	}); err != nil {
		logx.WithContext(l.ctx).Errorf("post ledger failed: roundId=%s type=%s status=%s err=%v",
			req.RoundId, typeCode, status, err)
	}
}

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

		// 1) 派彩账变（与回放、Outbox 事件同事务）
		if outcome.WinAmount > 0 {
			walletBalance := balance
			if _, err := l.svcCtx.Ledger.PostInTx(ctx, txConn, session, ledger.Posting{
				TypeCode:      model.TxTypeWin,
				MerchantID:    merchantID,
				MerchantCode:  req.MerchantId,
				UserID:        req.UserId,
				GameCode:      req.GameCode,
				RoundID:       req.RoundId,
				Amount:        outcome.WinAmount,
				WalletBalance: &walletBalance,
				Status:        model.LedgerStatusSuccess,
				RefType:       "game_round",
				RefID:         req.RoundId,
				Remark:        "结算派彩",
			}); err != nil {
				return fmt.Errorf("post win ledger: %w", err)
			}
		}

		// 2) 标记本地待处理表为已结算
		if err := model.NewPendingTransactionsModel(txConn).MarkSettled(ctx, merchantID, req.RoundId); err != nil {
			return fmt.Errorf("mark pending_transaction settled: %w", err)
		}

		// 3) 写入确定性回放
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

		// 4) 登记结算 Outbox 事件（多租户分区键）
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
		partitionKey := fmt.Sprintf("%s:%s", req.MerchantId, req.UserId)
		if err := l.svcCtx.Outbox.InsertInTx(ctx, session, &outbox.Record{
			ID:           settledID,
			Topic:        kafka.TopicRoundSettled,
			MerchantID:   merchantID,
			PartitionKey: partitionKey,
			Payload:      env,
		}); err != nil {
			return fmt.Errorf("outbox insert settled: %w", err)
		}

		// 5) 大奖广播单独投递
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
				PartitionKey: partitionKey,
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
