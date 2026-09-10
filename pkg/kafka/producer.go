package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

const TopicRoundSettled = "game.round.settled"

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

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    TopicRoundSettled,
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

	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", evt.UserID)),
		Value: body,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
