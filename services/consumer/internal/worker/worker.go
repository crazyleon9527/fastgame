package worker

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"fastgame/pkg/batch"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/kafka"
	"fastgame/pkg/rtpwatchdog"
	"fastgame/services/consumer/internal/svc"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type Worker struct {
	svcCtx *svc.ServiceContext
	reader *kafkago.Reader
	batch  *batch.Batcher[clickhouse.RoundSettledRow]
}

func NewWorker(svcCtx *svc.ServiceContext, maxSize int, interval time.Duration) *Worker {
	w := &Worker{svcCtx: svcCtx}
	w.batch = batch.NewBatcher(maxSize, interval, w.flush)
	w.reader = kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        svcCtx.Config.Kafka.Brokers,
		GroupID:        svcCtx.Config.Kafka.GroupID,
		Topic:          svcCtx.Config.Kafka.Topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	go w.batch.Start(ctx)

	for {
		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				w.batch.Stop()
				return ctx.Err()
			}
			logx.Errorf("kafka fetch: %v", err)
			continue
		}

		row, evt, err := w.parseMessage(msg.Value)
		if err != nil {
			logx.Errorf("parse message: %v", err)
			_ = w.reader.CommitMessages(ctx, msg)
			continue
		}

		if w.svcCtx.Config.RtpWatch.Enabled {
			w.evaluateRtp(ctx, evt)
		}

		if err := w.batch.Add(ctx, row); err != nil {
			logx.Errorf("batch add: %v", err)
			continue
		}

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			logx.Errorf("kafka commit: %v", err)
		}
	}
}

func (w *Worker) evaluateRtp(ctx context.Context, evt kafka.RoundSettledEvent) {
	alerts := w.svcCtx.Watchdog.Record(rtpwatchdog.RecordInput{
		MerchantCode: evt.MerchantID,
		GameCode:     evt.GameCode,
		UserID:       evt.UserID,
		BetMinor:     evt.BetAmount,
		WinMinor:     evt.WinAmount,
	})
	for _, alert := range alerts {
		if err := w.svcCtx.Enforcer.Handle(ctx, alert); err != nil {
			logx.Errorf("rtp enforcer: %v", err)
		}
	}
}

func (w *Worker) parseMessage(raw []byte) (clickhouse.RoundSettledRow, kafka.RoundSettledEvent, error) {
	var evt kafka.RoundSettledEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		return clickhouse.RoundSettledRow{}, evt, err
	}

	merchantID, err := w.resolveMerchantID(evt.MerchantID)
	if err != nil {
		return clickhouse.RoundSettledRow{}, evt, err
	}

	return clickhouse.RoundSettledRow{
		EventID:      evt.EventID,
		TraceID:      evt.TraceID,
		RoundID:      evt.RoundID,
		UserID:       evt.UserID,
		MerchantID:   merchantID,
		GameCode:     evt.GameCode,
		BetAmount:    evt.BetAmount,
		WinAmount:    evt.WinAmount,
		Multiplier:   evt.Multiplier,
		RtpTier:      evt.RtpTier,
		BalanceAfter: evt.Balance,
		SettledAt:    evt.SettledAt,
	}, evt, nil
}

func (w *Worker) resolveMerchantID(merchantIDOrCode string) (uint64, error) {
	if id, err := strconv.ParseUint(merchantIDOrCode, 10, 64); err == nil {
		return id, nil
	}

	merchant, err := w.svcCtx.Merchants.FindOneByMerchantCode(context.Background(), merchantIDOrCode)
	if err != nil {
		return 0, err
	}
	return merchant.Id, nil
}

func (w *Worker) flush(ctx context.Context, rows []clickhouse.RoundSettledRow) error {
	if len(rows) == 0 {
		return nil
	}
	logx.Infof("flushing %d round settled rows to clickhouse", len(rows))
	if err := w.svcCtx.Writer.BatchInsertRoundSettled(ctx, rows); err != nil {
		logx.Errorf("clickhouse batch insert failed: %v", err)
		return err
	}
	return nil
}

func (w *Worker) Close() error {
	if w.reader != nil {
		return w.reader.Close()
	}
	return nil
}
