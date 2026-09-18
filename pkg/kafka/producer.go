package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
			Addr:     kafka.TCP(brokers...),
			Balancer: &kafka.Hash{},
			// RequiredAcks 必须显式设置：kafka-go 的零值是 RequireNone（不等待 broker
			// 确认），消息可能静默丢失而无任何报错。RequireAll 让 broker 落盘后再返回，
			// 投递失败才可能被 outbox 捕获并重试。
			RequiredAcks: kafka.RequireAll,
			// 单条写入失败时重试次数（与 outbox 的退避重试互补：此处兜短暂网络抖动，
			// 仍失败则交给 outbox 的 next_retry_at）。
			MaxAttempts: 3,
			// 批量投递由 outbox 的 Dispatcher 控制节奏，这里缩短内部攒批时间，
			// 避免事件要等 writer 攒够一批才真正发出。
			BatchTimeout: 10 * time.Millisecond,
		},
	}
}

// PublishRaw 投递已经序列化好的消息体（供 outbox 派发器使用）。
// key 为空时由调用方保证稳定；此处不做二次封装，直接透传 body。
func (p *Producer) PublishRaw(ctx context.Context, topic, key string, body []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
	}
	if tid := trace.ID(ctx); tid != "" {
		msg.Headers = []kafka.Header{{Key: trace.HeaderTraceID, Value: []byte(tid)}}
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish raw to %s: %w", topic, err)
	}
	return nil
}

func (p *Producer) PublishRoundSettled(ctx context.Context, evt RoundSettledEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.SettledAt.IsZero() {
		evt.SettledAt = time.Now().UTC()
	}
	return p.publish(ctx, TopicRoundSettled, evt.UserID, evt)
}

func (p *Producer) PublishBigWin(ctx context.Context, evt BigWinEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}
	return p.publish(ctx, TopicEventBigwin, "", evt)
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
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}
	return p.publish(ctx, TopicWalletRollback, evt.UserID, evt)
}

func (p *Producer) publish(ctx context.Context, topic, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
	}
	if tid := trace.ID(ctx); tid != "" {
		msg.Headers = []kafka.Header{{Key: trace.HeaderTraceID, Value: []byte(tid)}}
	}
	return p.writer.WriteMessages(ctx, msg)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
