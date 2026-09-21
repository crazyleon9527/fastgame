package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"fastgame/pkg/async"
	applog "fastgame/pkg/log"
	"fastgame/pkg/money"
	"fastgame/pkg/wallet"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	ErrUserBusy       = errors.New("user has an ongoing action in progress for this game")
	ErrPluginNotFound = errors.New("game engine plugin not registered")
	ErrZeroBet        = errors.New("bet amount must be greater than zero")
)

type RedisLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

type DLQWriter interface {
	RecordDLQ(ctx context.Context, roundID, action, merchantID, userID string, amount money.Amount, reqPayload any, err error) error
}

type KafkaProducer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type UniversalHost struct {
	walletClient wallet.Client
	lock         RedisLock
	dlqWriter    DLQWriter
	producer     KafkaProducer
	settleTopic  string
	kafkaTasks   *async.Runner
}

func NewUniversalHost(
	walletClient wallet.Client,
	lock RedisLock,
	dlqWriter DLQWriter,
	producer KafkaProducer,
	settleTopic string,
) *UniversalHost {
	return &UniversalHost{
		walletClient: walletClient,
		lock:         lock,
		dlqWriter:    dlqWriter,
		producer:     producer,
		settleTopic:  settleTopic,
		kafkaTasks: async.New("engine-settle-dispatch",
			async.WithMaxConcurrency(engineDispatchConcurrency),
			async.WithDrainTimeout(engineDispatchDrainTimeout),
		),
	}
}

const (
	engineDispatchConcurrency  = 512
	engineDispatchDrainTimeout = 2 * time.Second
)

func (h *UniversalHost) Shutdown(ctx context.Context) error {
	if h == nil || h.kafkaTasks == nil {
		return nil
	}
	return h.kafkaTasks.Shutdown(ctx)
}

func (h *UniversalHost) DispatchStats() async.Stats {
	if h == nil || h.kafkaTasks == nil {
		return async.Stats{}
	}
	return h.kafkaTasks.Stats()
}

// ExecuteTurn 核心调度单局运转
func (h *UniversalHost) ExecuteTurn(ctx context.Context, in *TurnInput) (*TurnResult, error) {
	ctx = applog.WithRoundContext(ctx, applog.RoundFields{
		MerchantCode: in.MerchantCode,
		GameCode:     in.GameCode,
		RoundId:      in.RoundID,
		UserId:       in.UserID,
	})

	if in.BetAmount <= 0 {
		return nil, ErrZeroBet
	}

	// 1. 获取机台数学插件 (无锁读取快照)
	plugin, ok := GetPlugin(in.GameCode)
	if !ok {
		applog.C(ctx).Errorw("game_plugin_not_found", logx.Field(applog.KeyGameCode, in.GameCode))
		return nil, ErrPluginNotFound
	}

	// 2. 分布式机台锁 (粒度到 game，防止跨游戏无害操作互相阻塞)
	lockKey := fmt.Sprintf("lock:user:game:%s:%s:%s", in.MerchantCode, in.GameCode, in.UserID)
	locked, err := h.lock.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil || !locked {
		applog.C(ctx).Errorw("user_bet_concurrency_rejected",
			logx.Field("user_id", in.UserID),
			logx.Field(applog.KeyGameCode, in.GameCode),
		)
		return nil, ErrUserBusy
	}
	defer func() {
		// 使用独立超时释放锁，即使当前请求已超时退出也能正常解开分布式锁
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.lock.Release(releaseCtx, lockKey)
	}()

	// 3. 执行下注扣款 (Bet)
	betStart := time.Now()
	betReq := wallet.BetReq{
		MerchantID: in.MerchantID,
		UserID:     in.UserID,
		RoundID:    in.RoundID,
		Amount:     in.BetAmount,
	}
	betRes, err := h.walletClient.Bet(ctx, betReq)
	betLatency := time.Since(betStart).Milliseconds()

	if err != nil {
		applog.C(ctx).Errorw("seamless_wallet_bet_failed",
			logx.Field("duration_ms", betLatency),
			logx.Field(applog.KeyErr, err),
		)
		if wallet.IsTimeoutErr(err) && h.dlqWriter != nil {
			_ = h.dlqWriter.RecordDLQ(ctx, in.RoundID, "BET", in.MerchantID, in.UserID, in.BetAmount, betReq, err)
		}
		return nil, fmt.Errorf("wallet bet failed: %w", err)
	}

	currentBalance := betRes.Balance
	settlementStatus := "settled"

	// 4. 触发数学推演 (带 Panic 隔离保护)
	outcome, mathDurationUs, err := h.safeCalculateOutcome(ctx, plugin, in)
	if err != nil {
		applog.C(ctx).Errorw("math_plugin_panic_rollback",
			logx.Field("duration_us", mathDurationUs),
			logx.Field(applog.KeyErr, err),
		)
		// 执行退款 Rollback，并增加死信补偿保护
		rollbackReq := wallet.RollbackReq{
			MerchantID: in.MerchantID,
			UserID:     in.UserID,
			RoundID:    in.RoundID,
			Amount:     in.BetAmount,
			Reason:     "MATH_CALCULATION_ERROR",
		}
		if rbErr := h.walletClient.Rollback(ctx, rollbackReq); rbErr != nil {
			applog.C(ctx).Errorw("rollback_after_math_error_failed_entering_dlq",
				logx.Field(applog.KeyErr, rbErr),
				logx.Field(applog.KeyRoundID, in.RoundID),
			)
			if h.dlqWriter != nil {
				_ = h.dlqWriter.RecordDLQ(ctx, in.RoundID, "ROLLBACK", in.MerchantID, in.UserID, in.BetAmount, rollbackReq, rbErr)
			}
		}
		return nil, errors.New("system computation error, bet refunded")
	}

	// 5. 执行派彩扣款 (Win)
	var winLatency int64
	if outcome.WinAmount > 0 {
		winStart := time.Now()
		winReq := wallet.WinReq{
			MerchantID: in.MerchantID,
			UserID:     in.UserID,
			RoundID:    in.RoundID,
			Amount:     outcome.WinAmount,
		}
		winRes, err := h.walletClient.Win(ctx, winReq)
		winLatency = time.Since(winStart).Milliseconds()

		if err != nil {
			applog.C(ctx).Errorw("seamless_wallet_win_failed_entering_dlq",
				logx.Field("duration_ms", winLatency),
				logx.Field("win_amount", outcome.WinAmount.Minor()),
				logx.Field(applog.KeyErr, err),
			)
			if h.dlqWriter != nil {
				_ = h.dlqWriter.RecordDLQ(ctx, in.RoundID, "WIN", in.MerchantID, in.UserID, outcome.WinAmount, winReq, err)
			}
			settlementStatus = "pending"
		} else {
			currentBalance = winRes.Balance
		}
	}

	// 6. 异步派发落盘事件
	h.asyncDispatchSettledEvent(ctx, in, outcome, int(betLatency+winLatency), int(mathDurationUs), settlementStatus)

	return &TurnResult{
		RoundID:             in.RoundID,
		Balance:             currentBalance,
		WinAmount:           outcome.WinAmount,
		PayoutMultiplier:    outcome.PayoutMultiplier,
		SettlementStatus:    settlementStatus,
		PresentationPayload: outcome.PresentationPayload,
	}, nil
}

// safeCalculateOutcome 带 Panic 捕获的数学推演调用
func (h *UniversalHost) safeCalculateOutcome(ctx context.Context, plugin GamePlugin, in *TurnInput) (out *TurnOutcome, durationUs int64, err error) {
	start := time.Now()
	defer func() {
		durationUs = time.Since(start).Microseconds()
		if r := recover(); r != nil {
			err = fmt.Errorf("math plugin panic: %v, stack: %s", r, string(debug.Stack()))
		}
	}()

	out, err = plugin.CalculateOutcome(ctx, in)
	return out, durationUs, err
}

type SettledEventPayload struct {
	RoundID             string  `json:"round_id"`
	MerchantCode        string  `json:"merchant_code"`
	GameCode            string  `json:"game_code"`
	UserID              string  `json:"user_id"`
	Currency            string  `json:"currency"`
	BetAmount           int64   `json:"bet_amount"`
	WinAmount           int64   `json:"win_amount"`
	NetProfit           int64   `json:"net_profit"`
	PayoutMultiplier    float64 `json:"payout_multiplier"`
	MathVersion         string  `json:"math_version"`
	RtpTierApplied      float64 `json:"rtp_tier_applied"`
	SettlementStatus    string  `json:"settlement_status"`
	ServerSeed          string  `json:"server_seed"`
	ServerSeedHash      string  `json:"server_seed_hash"`
	ClientSeed          string  `json:"client_seed"`
	Nonce               uint64  `json:"nonce"`
	PresentationPayload string  `json:"presentation_payload"`
	ExecutionTimeUs     int     `json:"execution_time_us"`
	WalletLatencyMs     int     `json:"wallet_latency_ms"`
	CreatedAt           string  `json:"created_at"`
}

func (h *UniversalHost) asyncDispatchSettledEvent(
	ctx context.Context,
	in *TurnInput,
	out *TurnOutcome,
	walletLatencyMs int,
	mathDurationUs int,
	status string,
) {
	if h.producer == nil {
		return
	}

	event := SettledEventPayload{
		RoundID:             in.RoundID,
		MerchantCode:        in.MerchantCode,
		GameCode:            in.GameCode,
		UserID:              in.UserID,
		Currency:            in.Currency,
		BetAmount:           in.BetAmount.Minor(),
		WinAmount:           out.WinAmount.Minor(),
		NetProfit:           in.BetAmount.Minor() - out.WinAmount.Minor(),
		PayoutMultiplier:    out.PayoutMultiplier,
		MathVersion:         out.MathVersion,
		RtpTierApplied:      out.RtpApplied,
		SettlementStatus:    status,
		ServerSeed:          out.ServerSeed,
		ServerSeedHash:      out.ServerSeedHash,
		ClientSeed:          out.ClientSeed,
		Nonce:               out.Nonce,
		PresentationPayload: out.PresentationPayload,
		ExecutionTimeUs:     mathDurationUs,
		WalletLatencyMs:     walletLatencyMs,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := json.Marshal(event)
	if err != nil {
		return
	}

	msg := kafka.Message{
		Topic: h.settleTopic,
		Key:   []byte(fmt.Sprintf("%s:%s", in.MerchantCode, in.UserID)),
		Value: raw,
	}
	applog.InjectKafkaHeader(ctx, &msg)

	h.kafkaTasks.Go(ctx, func(taskCtx context.Context) {
		dispatchCtx, cancel := context.WithTimeout(taskCtx, 2*time.Second)
		defer cancel()
		if err := h.producer.WriteMessages(dispatchCtx, msg); err != nil {
			applog.C(taskCtx).Errorw("kafka_dispatch_settle_failed",
				logx.Field(applog.KeyErr, err),
				logx.Field(applog.KeyRoundID, in.RoundID),
			)
		}
	})
}
