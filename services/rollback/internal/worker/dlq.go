package worker

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/kafka"
	"fastgame/services/rollback/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func publishReconcileDLQ(ctx context.Context, svcCtx *svc.ServiceContext, evt kafka.ReconcileDLQEvent) {
	if svcCtx.Kafka == nil {
		return
	}
	if evt.FailedAt.IsZero() {
		evt.FailedAt = time.Now().UTC()
	}
	if err := svcCtx.Kafka.PublishReconcileDLQ(ctx, evt); err != nil {
		logx.Errorf("publish reconcile dlq failed: source=%s recordId=%d err=%v", evt.Source, evt.RecordID, err)
	}
}

func dlqFromPendingOp(op *model.WalletPendingOp, lastError string) kafka.ReconcileDLQEvent {
	return kafka.ReconcileDLQEvent{
		Source:       "wallet_pending_ops",
		RecordID:     op.Id,
		RoundID:      op.RoundID,
		MerchantCode: op.MerchantCode,
		UserID:       op.UserID,
		OpType:       op.OpType,
		RetryCount:   op.RetryCount,
		LastError:    lastError,
	}
}

func dlqFromPendingTx(tx *model.PendingTransaction, lastError string) kafka.ReconcileDLQEvent {
	return kafka.ReconcileDLQEvent{
		Source:       "pending_transactions",
		RecordID:     tx.Id,
		RoundID:      tx.RoundID,
		MerchantCode: tx.MerchantCode,
		UserID:       tx.UserID,
		OpType:       tx.Phase,
		RetryCount:   tx.RetryCount,
		LastError:    lastError,
	}
}
