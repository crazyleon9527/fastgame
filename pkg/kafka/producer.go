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
)

type RoundSettledEvent struct {
	EventID    string    `json:"eventId"`
	TraceID    string    `json:"traceId"`
	RoundID    string    `json:"roundId"`
	UserID     uint64    `json:"userId"`
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
	UserID     uint64    `json:"userId"`
	MerchantID string    `json:"merchantId"`
	GameCode   string    `json:"gameCode"`
	WinAmount  int64     `json:"winAmount"`
	Multiplier int64     `json:"multiplier"`
	OccurredAt time.Time `json:"occurredAt"`
}

type WalletRollbackEvent struct {
	EventID      string    `json:"eventId"`
	TraceID      string    `json:"traceId"`
	RoundID      string    `json:"roundId"`
	UserID       uint64    `json:"userId"`
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
		},
	}
}

func (p *Producer) PublishRoundSettled(ctx context.Context, evt RoundSettledEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.SettledAt.IsZero() {
		evt.SettledAt = time.Now().UTC()
	}
	return p.publish(ctx, TopicRoundSettled, fmt.Sprintf("%d", evt.UserID), evt)
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

func (p *Producer) PublishWalletRollback(ctx context.Context, evt WalletRollbackEvent) error {
	if evt.EventID == "" {
		evt.EventID = uuid.NewString()
	}
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = time.Now().UTC()
	}
	return p.publish(ctx, TopicWalletRollback, fmt.Sprintf("%d", evt.UserID), evt)
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
