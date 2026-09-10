package worker

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/wallet"
	"fastgame/services/rollback/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	orphanScanInterval = 30 * time.Second
	orphanMinAge       = time.Minute
	orphanMaxRetries   = 10
)

type OrphanReconciler struct {
	svcCtx *svc.ServiceContext
}

func NewOrphanReconciler(svcCtx *svc.ServiceContext) *OrphanReconciler {
	return &OrphanReconciler{svcCtx: svcCtx}
}

func (r *OrphanReconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(orphanScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.process(ctx)
		}
	}
}

func (r *OrphanReconciler) process(ctx context.Context) {
	txs, err := r.svcCtx.PendingTx.ListStalePending(ctx, orphanMinAge, 50)
	if err != nil {
		logx.Errorf("orphan reconciler list: %v", err)
		return
	}

	for _, tx := range txs {
		if tx.RetryCount >= orphanMaxRetries {
			if err := r.svcCtx.PendingTx.MarkFailed(ctx, tx.Id, "max retries exceeded"); err != nil {
				logx.Errorf("orphan mark failed: id=%d err=%v", tx.Id, err)
			}
			continue
		}
		r.reconcileOne(ctx, tx)
	}
}

func (r *OrphanReconciler) reconcileOne(ctx context.Context, tx *model.PendingTransaction) {
	check, err := r.svcCtx.Wallet.CheckTransaction(ctx, tx.MerchantCode, tx.UserID, tx.RoundID)
	if err != nil {
		r.bumpRetry(ctx, tx.Id, err.Error())
		return
	}

	switch check.Status {
	case wallet.TxStatusSettled:
		r.markDone(ctx, tx, "wallet already settled")

	case wallet.TxStatusRolledBack:
		r.markDone(ctx, tx, "wallet already rolled back")

	case wallet.TxStatusBetOnly:
		if tx.WinAmount > 0 && tx.ExpectedAction == model.PendingTxActionSettleWin {
			r.retryWin(ctx, tx)
			return
		}
		r.rollbackBet(ctx, tx)

	case wallet.TxStatusNotFound:
		if tx.Phase == model.PendingTxPhaseBetDebited || tx.BetAmount > 0 {
			r.rollbackBet(ctx, tx)
			return
		}
		r.markDone(ctx, tx, "wallet not found, local orphan cleared")

	default:
		r.bumpRetry(ctx, tx.Id, "unknown wallet status: "+check.Status)
	}
}

func (r *OrphanReconciler) retryWin(ctx context.Context, tx *model.PendingTransaction) {
	_, err := r.svcCtx.Wallet.Win(ctx, wallet.WinReq{
		MerchantID: tx.MerchantCode,
		UserID:     tx.UserID,
		RoundID:    tx.RoundID,
		Amount:     tx.WinAmount,
	})
	if err != nil {
		r.bumpRetry(ctx, tx.Id, err.Error())
		return
	}
	r.markDone(ctx, tx, "win compensated")
}

func (r *OrphanReconciler) rollbackBet(ctx context.Context, tx *model.PendingTransaction) {
	err := r.svcCtx.Wallet.Rollback(ctx, wallet.RollbackReq{
		MerchantID: tx.MerchantCode,
		UserID:     tx.UserID,
		RoundID:    tx.RoundID,
		Amount:     tx.BetAmount,
		Reason:     "orphan_reconcile",
	})
	if err != nil {
		r.bumpRetry(ctx, tx.Id, err.Error())
		return
	}
	r.markDone(ctx, tx, "bet rolled back")
}

func (r *OrphanReconciler) markDone(ctx context.Context, tx *model.PendingTransaction, detail string) {
	if err := r.svcCtx.PendingTx.MarkDone(ctx, tx.Id); err != nil {
		logx.Errorf("orphan mark done: id=%d err=%v", tx.Id, err)
		return
	}
	logx.Infof("orphan reconciled: roundId=%s traceId=%s detail=%s", tx.RoundID, tx.TraceID, detail)
}

func (r *OrphanReconciler) bumpRetry(ctx context.Context, id uint64, msg string) {
	if err := r.svcCtx.PendingTx.IncrementRetry(ctx, id, msg); err != nil {
		logx.Errorf("orphan increment retry: id=%d err=%v", id, err)
	}
}
