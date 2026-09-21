package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	applog "fastgame/pkg/log"
	"fastgame/pkg/trace"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

const (
	TopicRoundSettled   = "game.round.settled"
	TopicWalletRollback = "game.wallet.rollback"
	TopicEventBigwin    = "game.event.bigwin"
	TopicReconcileDLQ   = "game.reconcile.dlq"
)

type RoundSettledEvent struct {
	EventID    string    `json:"eventId"`
	TraceID    string    `json:"traceId"`
	RoundID    string    `json:"roundId"`
	UserID     string    `json:"userId"`
	MerchantID string    `json:"merchantId"`
	GameCode   string    `json:"gameCode"`
	BetAmount  int64     `json:"betAmount"`
	WinAmount  int64     `json:"winAmount"`
	Multiplier int64     `json:"multiplier"`
	RtpTier    string    `json:"rtpTier"`
	Balance    int64     `json:"balanceAfter"`
	SettledAt  time.Time `json:"settledAt"`
}

type BigWinEvent struct {
	EventID    string    `json:"eventId"`
	TraceID    string    `json:"traceId,omitempty"`
	RoundID    string    `json:"roundId"`
	UserID     string    `json:"userId"`
	MerchantID string    `json:"merchantId"`
	GameCode   string    `json:"gameCode"`
	WinAmount  int64     `json:"winAmount"`
	Multiplier int64     `json:"multiplier"`
	OccurredAt time.Time `json:"occurredAt"`
}

type ReconcileDLQEvent struct {
	Source       string    `json:"source"`
	RecordID     uint64    `json:"recordId"`
	RoundID      string    `json:"roundId"`
	MerchantCode string    `json:"merchantCode"`
	UserID       string    `json:"userId,omitempty"`
	OpType       string    `json:"opType"`
	RetryCount   int64     `json:"retryCount"`
	LastError    string    `json:"lastError"`
	FailedAt     time.Time `json:"failedAt"`
}

type WalletRollbackEvent struct {
	EventID      string    `json:"eventId"`
	TraceID      string    `json:"traceId"`
	RoundID      string    `json:"roundId"`
	UserID       string    `json:"userId"`
	MerchantID   string    `json:"merchantId"`
	RollbackType string    `json:"rollbackType"`
	Amount       int64     `json:"amount"`
	Reason       string    `json:"reason"`
	OccurredAt   time.Time `json:"occurredAt"`
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr: kafka.TCP(brokers...),
			// 混合平衡器：当 Key 不为空时使用 Hash（保证同一用户/注单顺序），
			// 当 Key 为空时自动回退为最小负载/轮询策略，避免全打到 0 号分区
			Balancer: &kafka.Hash{},
			// 强可靠性落盘确认：要求所有 ISR 副本落盘才返回，防止高并发下静默丢消息
			RequiredAcks: kafka.RequireAll,
			// 启用 Snappy 高性能压缩：大幅降低注单 JSON 的网络 I/O 开销与 Kafka 磁盘占用
			Compression: kafka.Snappy,
			// 重试机制：配合应用层兜底瞬间的网络抖动
			MaxAttempts: 3,
			// 攒批延时：10ms 平衡了极低延迟与合理的网络吞吐
			BatchTimeout: 10 * time.Millisecond,
			// 硬超时保护：防止 Broker 不可用时导致业务 Goroutine 挂起
			WriteTimeout: 5 * time.Second,
			ReadTimeout:  5 * time.Second,
		},
	}
}

// PublishRaw 投递已经序列化好的消息体（供 outbox 派发器等直接透传使用）
func (p *Producer) PublishRaw(ctx context.Context, topic, key string, body []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
	}
	applog.InjectKafkaHeader(ctx, &msg)

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish raw to %s: %w", topic, err)
	}
	return nil
}

func (p *Producer) PublishRoundSettled(ctx context.Context, evt RoundSettledEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.TraceID == "" {
		evt.TraceID = trace.ID(ctx)
	}
	if evt.SettledAt.IsZero() {
		evt.SettledAt = time.Now().UTC()
	}
	// 按 UserID 作为 Key，保证同一玩家的所有注单事件在同一个 Partition 内严格保序
	return p.publish(ctx, TopicRoundSettled, evt.UserID, evt)
}

func (p *Producer) PublishBigWin(ctx context.Context, evt BigWinEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.TraceID == "" {
		evt.TraceID = trace.ID(ctx)
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}
	// 广播事件使用 RoundID 或随机 Key，均匀打散到所有分区
	key := evt.RoundID
	if key == "" {
		key = evt.EventID
	}
	return p.publish(ctx, TopicEventBigwin, key, evt)
}

func (p *Producer) PublishReconcileDLQ(ctx context.Context, evt ReconcileDLQEvent) error {
	if evt.FailedAt.IsZero() {
		evt.FailedAt = time.Now().UTC()
	}
	key := evt.RoundID
	if key == "" {
		key = fmt.Sprintf("%d", evt.RecordID)
	}
	return p.publish(ctx, TopicReconcileDLQ, key, evt)
}

func (p *Producer) PublishWalletRollback(ctx context.Context, evt WalletRollbackEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.TraceID == "" {
		evt.TraceID = trace.ID(ctx)
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}
	return p.publish(ctx, TopicWalletRollback, evt.UserID, evt)
}

func (p *Producer) publish(ctx context.Context, topic, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kafka marshal payload for %s: %w", topic, err)
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
	}
	// 注入 Trace 上下文
	applog.InjectKafkaHeader(ctx, &msg)

	return p.writer.WriteMessages(ctx, msg)
}

func (p *Producer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
