package worker

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/wallet"
	"fastgame/services/rollback/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const maxRetries = 10

type Reconciler struct {
	svcCtx *svc.ServiceContext
}

func NewReconciler(svcCtx *svc.ServiceContext) *Reconciler {
	return &Reconciler{svcCtx: svcCtx}
}

func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processPending(ctx)
		}
	}
}

func (r *Reconciler) processPending(ctx context.Context) {
	ops, err := r.svcCtx.PendingOps.ListPending(ctx, 50)
	if err != nil {
		logx.Errorf("list pending ops: %v", err)
		return
	}

	for _, op := range ops {
		if op.RetryCount >= maxRetries {
			if err := r.svcCtx.PendingOps.MarkFailed(ctx, op.Id, "max retries exceeded"); err != nil {
				logx.Errorf("mark failed: id=%d err=%v", op.Id, err)
			}
			continue
		}

		switch op.OpType {
		case model.PendingOpWinFailed, model.PendingOpWinTimeout:
			r.retryRollback(ctx, op)
		case model.PendingOpRollback:
			r.retryRollback(ctx, op)
		default:
			logx.Errorf("unknown pending op type: %s", op.OpType)
		}
	}
}

func (r *Reconciler) retryRollback(ctx context.Context, op *model.WalletPendingOp) {
	err := r.svcCtx.Wallet.Rollback(ctx, wallet.RollbackReq{
		MerchantID: op.MerchantCode,
		UserID:     op.UserID,
		RoundID:    op.RoundID,
		Amount:     op.BetAmount,
		Reason:     op.OpType,
	})
	if err != nil {
		if incErr := r.svcCtx.PendingOps.IncrementRetry(ctx, op.Id, err.Error()); incErr != nil {
			logx.Errorf("increment retry: id=%d err=%v", op.Id, incErr)
		}
		return
	}

	if err := r.svcCtx.PendingOps.MarkDone(ctx, op.Id); err != nil {
		logx.Errorf("mark done: id=%d err=%v", op.Id, err)
	}
	logx.Infof("reconciled pending op: roundId=%s type=%s", op.RoundID, op.OpType)
}
