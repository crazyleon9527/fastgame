package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fastgame/pkg/async"
	applog "fastgame/pkg/log"
	"fastgame/pkg/money"
	"fastgame/pkg/wallet"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	ErrUserBusy       = errors.New("user has an ongoing bet in progress")
	ErrPluginNotFound = errors.New("game engine plugin not registered")
	ErrZeroBet        = errors.New("bet amount must be greater than zero")
)

// RedisLock 分布式排他锁抽象 (复用 pkg/lock)
type RedisLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

// DLQWriter 钱包死信队列写入抽象
type DLQWriter interface {
	RecordDLQ(ctx context.Context, roundID, action, merchantID, userID string, amount money.Amount, reqPayload any, err error) error
}

// KafkaProducer 异步消息生产抽象
type KafkaProducer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

// UniversalHost 全机台通用的执行宿主
type UniversalHost struct {
	walletClient wallet.Client
	lock         RedisLock
	dlqWriter    DLQWriter
	producer     KafkaProducer
	settleTopic  string
	// kafkaTasks 负责结算事件的异步投递：并发闸门 + panic 兜底 + 可 Drain。
	// 原来这里是裸 go func，投递任务没有任何上限，panic 会直接打挂进程，
	// 进程退出时在途投递被静默丢弃（结算事件丢了却没人在日志里看到）。
	kafkaTasks *async.Runner
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
	// engineDispatchConcurrency 限制同时在途的结算事件投递数，
	// 避免 Kafka 抖动时无人限制地堆积 goroutine。
	engineDispatchConcurrency  = 512
	engineDispatchDrainTimeout = 2 * time.Second
)

// Shutdown 等待在途的结算事件投递完成，供进程优雅退出时调用。
func (h *UniversalHost) Shutdown(ctx context.Context) error {
	if h == nil || h.kafkaTasks == nil {
		return nil
	}
	return h.kafkaTasks.Shutdown(ctx)
}

// DispatchStats 暴露投递侧计数（被拒/panic），便于监控结算事件是否在丢弃。
func (h *UniversalHost) DispatchStats() async.Stats {
	if h == nil || h.kafkaTasks == nil {
		return async.Stats{}
	}
	return h.kafkaTasks.Stats()
}

// ExecuteTurn 核心调度单局运转
func (h *UniversalHost) ExecuteTurn(ctx context.Context, in *TurnInput) (*TurnResult, error) {
	// 1. 注入全链路审计上下文
	ctx = applog.WithRoundContext(ctx, applog.RoundFields{
		MerchantCode: in.MerchantCode,
		GameCode:     in.GameCode,
		RoundId:      in.RoundID,
		UserId:       in.UserID,
	})

	if in.BetAmount <= 0 {
		return nil, ErrZeroBet
	}

	// 2. 路由目标机台数学插件 (无 I/O 前置检查)
	plugin, ok := GetPlugin(in.GameCode)
	if !ok {
		applog.C(ctx).Errorw("game_plugin_not_found", logx.Field(applog.KeyGameCode, in.GameCode))
		return nil, ErrPluginNotFound
	}

	// 3. 用户排他分布式锁 (防止单玩家连点并发双花)
	lockKey := fmt.Sprintf("lock:user:%s:%s", in.MerchantCode, in.UserID)
	locked, err := h.lock.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil || !locked {
		applog.C(ctx).Errorw("user_bet_concurrency_rejected", logx.Field("user_id", in.UserID))
		return nil, ErrUserBusy
	}
	defer func() {
		_ = h.lock.Release(ctx, lockKey)
	}()

	// 4. 调用三方无缝钱包执行原子扣款 (Bet)
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
		// 若遇到网络不可逆超时，异步登记死信队列补偿
		if wallet.IsTimeoutErr(err) && h.dlqWriter != nil {
			_ = h.dlqWriter.RecordDLQ(ctx, in.RoundID, "BET", in.MerchantID, in.UserID, in.BetAmount, betReq, err)
		}
		return nil, fmt.Errorf("wallet bet failed: %w", err)
	}

	currentBalance := betRes.Balance

	// 5. 触发游戏专属数学插件推演 (纯内存，目标耗时 < 50us)
	mathStart := time.Now()
	outcome, err := plugin.CalculateOutcome(ctx, in)
	mathDurationUs := time.Since(mathStart).Microseconds()

	if err != nil {
		// 数学推演极端崩溃异常: 必须立即执行 Rollback 撤销扣款
		applog.C(ctx).Errorw("math_plugin_panic_rollback",
			logx.Field("duration_us", mathDurationUs),
			logx.Field(applog.KeyErr, err),
		)
		_ = h.walletClient.Rollback(ctx, wallet.RollbackReq{
			MerchantID: in.MerchantID,
			UserID:     in.UserID,
			RoundID:    in.RoundID,
			Amount:     in.BetAmount,
			Reason:     "MATH_CALCULATION_ERROR",
		})
		return nil, errors.New("system computation error, bet refunded")
	}

	// 6. 调用三方无缝钱包执行原子派彩 (Win，允许派彩金额为 0)
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
			// 派彩超时绝对不可回滚单局，必须强制落死信队列等待重试自愈
			if h.dlqWriter != nil {
				_ = h.dlqWriter.RecordDLQ(ctx, in.RoundID, "WIN", in.MerchantID, in.UserID, outcome.WinAmount, winReq, err)
			}
			// 虽然后台记了死信，但在当前管道中先将预估余额累加返回给用户保证前端动效不卡死
			currentBalance += outcome.WinAmount
		} else {
			currentBalance = winRes.Balance
		}
	}

	// 7. 组装注单落盘事实，异步投递 Kafka (解耦 ClickHouse 写入压力)
	h.asyncDispatchSettledEvent(ctx, in, outcome, int(betLatency+winLatency), int(mathDurationUs))

	// 8. 返回前端所需的极简数据结构
	return &TurnResult{
		RoundID:             in.RoundID,
		Balance:             currentBalance,
		WinAmount:           outcome.WinAmount,
		PayoutMultiplier:    outcome.PayoutMultiplier,
		PresentationPayload: outcome.PresentationPayload,
	}, nil
}

// 异步投递注单事件
func (h *UniversalHost) asyncDispatchSettledEvent(
	ctx context.Context,
	in *TurnInput,
	out *TurnOutcome,
	walletLatencyMs int,
	mathDurationUs int,
) {
	if h.producer == nil {
		return
	}

	// 构造发往 ClickHouse 消费组的事件结构
	eventPayload := map[string]any{
		"round_id":             in.RoundID,
		"merchant_code":        in.MerchantCode,
		"game_code":            in.GameCode,
		"user_id":              in.UserID,
		"currency":             in.Currency,
		"bet_amount":           in.BetAmount.Minor(),
		"win_amount":           out.WinAmount.Minor(),
		"net_profit":           in.BetAmount.Minor() - out.WinAmount.Minor(),
		"payout_multiplier":    out.PayoutMultiplier,
		"math_version":         out.MathVersion,
		"rtp_tier_applied":     out.RtpApplied,
		"server_seed":          out.ServerSeed,
		"server_seed_hash":     out.ServerSeedHash,
		"client_seed":          out.ClientSeed,
		"nonce":                out.Nonce,
		"presentation_payload": out.PresentationPayload,
		"execution_time_us":    mathDurationUs,
		"wallet_latency_ms":    walletLatencyMs,
		"created_at":           time.Now().UTC().Format(time.RFC3339),
	}

	raw, _ := json.Marshal(eventPayload)
	msg := kafka.Message{
		Topic: h.settleTopic,
		Key:   []byte(in.RoundID),
		Value: raw,
	}
	// 注入链路追踪 Header
	applog.InjectKafkaHeader(ctx, &msg)

	// 结算事件投递交给受管 Runner：
	//   - Detach 保留 trace_id 但摘掉请求取消（主请求退出不应导致 Kafka 丢包）；
	//   - 入队失败（积压到上限或进程退出中）会记 warn，不再静默丢事件。
	enqueued := h.kafkaTasks.Go(ctx, func(taskCtx context.Context) {
		dispatchCtx, cancel := context.WithTimeout(taskCtx, 2*time.Second)
		defer cancel()
		if err := h.producer.WriteMessages(dispatchCtx, msg); err != nil {
			applog.C(taskCtx).Errorw("kafka_dispatch_settle_failed",
				logx.Field(applog.KeyErr, err),
				logx.Field(applog.KeyRoundID, in.RoundID),
			)
		}
	})
	if !enqueued {
		applog.C(ctx).Errorw("kafka_dispatch_settle_rejected",
			logx.Field(applog.KeyRoundID, in.RoundID),
			logx.Field("running", h.kafkaTasks.Running()),
		)
	}
}
