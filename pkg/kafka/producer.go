package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	RoundID    string    `json:"roundId"`
	UserID     uint64    `json:"userId"`
	MerchantID string    `json:"merchantId"`
	GameCode   string    `json:"gameCode"`
	BetAmount  float64   `json:"betAmount"`
	WinAmount  float64   `json:"winAmount"`
	Multiplier float64   `json:"multiplier"`
	RtpTier    string    `json:"rtpTier"`
	Balance    float64   `json:"balanceAfter"`
	SettledAt  time.Time `json:"settledAt"`
}

type BigWinEvent struct {
	EventID    string    `json:"eventId"`
	RoundID    string    `json:"roundId"`
	UserID     uint64    `json:"userId"`
	MerchantID string    `json:"merchantId"`
	GameCode   string    `json:"gameCode"`
	WinAmount  float64   `json:"winAmount"`
	Multiplier float64   `json:"multiplier"`
	OccurredAt time.Time `json:"occurredAt"`
}

type WalletRollbackEvent struct {
	EventID      string    `json:"eventId"`
	RoundID      string    `json:"roundId"`
	UserID       uint64    `json:"userId"`
	MerchantID   string    `json:"merchantId"`
	RollbackType string    `json:"rollbackType"`
	Amount       float64   `json:"amount"`
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
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
