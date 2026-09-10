package worker

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"fastgame/pkg/clickhouse"
	"fastgame/pkg/kafka"
	"fastgame/pkg/wallet"
	"fastgame/services/rollback/internal/svc"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type Worker struct {
	svcCtx *svc.ServiceContext
	reader *kafkago.Reader
}

func NewWorker(svcCtx *svc.ServiceContext) *Worker {
	return &Worker{
		svcCtx: svcCtx,
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        svcCtx.Config.Kafka.Brokers,
			GroupID:        svcCtx.Config.Kafka.GroupID,
			Topic:          svcCtx.Config.Kafka.Topic,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
		}),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	go NewReconciler(w.svcCtx).Run(ctx)
	go NewOrphanReconciler(w.svcCtx).Run(ctx)

	for {
		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logx.Errorf("kafka fetch: %v", err)
			continue
		}

		if err := w.handleMessage(ctx, msg.Value); err != nil {
			logx.Errorf("handle rollback message failed (will retry): %v", err)
			continue
		}

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			logx.Errorf("kafka commit: %v", err)
		}
	}
}

func (w *Worker) handleMessage(ctx context.Context, raw []byte) error {
	var evt kafka.WalletRollbackEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		return err
	}

	merchantID, err := w.resolveMerchantID(evt.MerchantID)
	if err != nil {
		return err
	}

	status := "done"
	rollbackErr := w.svcCtx.Wallet.Rollback(ctx, wallet.RollbackReq{
		MerchantID: evt.MerchantID,
		UserID:     evt.UserID,
		RoundID:    evt.RoundID,
		Amount:     evt.Amount,
		Reason:     evt.Reason,
	})
	if rollbackErr != nil {
		logx.Errorf("wallet rollback call failed: roundId=%s err=%v", evt.RoundID, rollbackErr)
		status = "pending"
	}

	chErr := w.svcCtx.Writer.BatchInsertWalletRollback(ctx, []clickhouse.WalletRollbackRow{{
		EventID:      evt.EventID,
		TraceID:      evt.TraceID,
		RoundID:      evt.RoundID,
		UserID:       evt.UserID,
		MerchantID:   merchantID,
		RollbackType: evt.RollbackType,
		Amount:       evt.Amount,
		Reason:       evt.Reason,
		Status:       status,
		OccurredAt:   evt.OccurredAt,
	}})
	if chErr != nil {
		return chErr
	}
	return rollbackErr
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

func (w *Worker) Close() error {
	if w.reader != nil {
		return w.reader.Close()
	}
	return nil
}
