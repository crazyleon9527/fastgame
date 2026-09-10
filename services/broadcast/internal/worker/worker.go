package worker

import (
	"context"
	"encoding/json"
	"time"

	"fastgame/pkg/kafka"
	"fastgame/services/broadcast/internal/hub"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type Worker struct {
	hub    *hub.Hub
	reader *kafkago.Reader
}

func NewWorker(brokers []string, groupID string, h *hub.Hub) *Worker {
	return &Worker{
		hub: h,
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        brokers,
			GroupID:        groupID,
			Topic:          kafka.TopicEventBigwin,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
		}),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	for {
		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logx.Errorf("kafka fetch: %v", err)
			continue
		}

		var evt kafka.BigWinEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			logx.Errorf("parse bigwin: %v", err)
		} else {
			w.hub.Broadcast(map[string]any{
				"type":       "big_win",
				"roundId":    evt.RoundID,
				"userId":     evt.UserID,
				"merchantId": evt.MerchantID,
				"gameCode":   evt.GameCode,
				"winAmount":  evt.WinAmount,
				"multiplier": evt.Multiplier,
				"occurredAt": evt.OccurredAt,
			})
		}

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			logx.Errorf("kafka commit: %v", err)
		}
	}
}

func (w *Worker) Close() error {
	if w.reader != nil {
		return w.reader.Close()
	}
	return nil
}
