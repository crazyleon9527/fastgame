package worker

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/ledger"
	"fastgame/pkg/money"
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
			lastErr := "max retries exceeded"
			if op.LastError.Valid {
				lastErr = op.LastError.String
			}
			publishReconcileDLQ(ctx, r.svcCtx, dlqFromPendingOp(op, lastErr))
			if err := r.svcCtx.PendingOps.MarkFailed(ctx, op.Id, "max retries exceeded"); err != nil {
				logx.Errorf("mark failed: id=%d err=%v", op.Id, err)
			}
			continue
		}

		switch op.OpType {
		case model.PendingOpWinFailed, model.PendingOpWinTimeout:
			r.retryWin(ctx, op)
		case model.PendingOpRollback:
			r.retryRollback(ctx, op)
		default:
			logx.Errorf("unknown pending op type: %s", op.OpType)
		}
	}
}

func (r *Reconciler) retryWin(ctx context.Context, op *model.WalletPendingOp) {
	result, err := r.svcCtx.Wallet.Win(ctx, wallet.WinReq{
		MerchantID: op.MerchantCode,
		UserID:     op.UserID,
		RoundID:    op.RoundID,
		Amount:     money.AmountFromMinor(op.WinAmount),
	})
	if err != nil {
		if incErr := r.svcCtx.PendingOps.IncrementRetry(ctx, op.Id, err.Error()); incErr != nil {
			logx.Errorf("increment retry: id=%d err=%v", op.Id, incErr)
		}
		return
	}

	// 补偿成功 = 钱真的到账了，必须在账本上补一条 SUCCESS 流水。
	// 首次失败时 RGS 记的是 (merchant, round, WIN, PENDING_RETRY)，
	// 幂等键带终态，所以这条 SUCCESS 不会被当成重复丢掉。
	// 以钱包返回的余额为准做快照，可顺带发现本地镜像与钱包的偏差。
	walletBalance := result.Balance
	postLedger(ctx, r.svcCtx, ledgerPosting{
		merchantID:    op.MerchantID,
		merchantCode:  op.MerchantCode,
		userID:        op.UserID,
		roundID:       op.RoundID,
		typeCode:      model.TxTypeWin,
		amount:        op.WinAmount,
		walletBalance: &walletBalance,
		remark:        "补偿重试派彩成功",
		refType:       "wallet_pending_op",
		refID:         op.OpType,
	})

	if err := r.svcCtx.PendingOps.MarkDone(ctx, op.Id); err != nil {
		logx.Errorf("mark done: id=%d err=%v", op.Id, err)
	}
	logx.Infof("reconciled win op: roundId=%s type=%s", op.RoundID, op.OpType)
}

func (r *Reconciler) retryRollback(ctx context.Context, op *model.WalletPendingOp) {
	err := r.svcCtx.Wallet.Rollback(ctx, wallet.RollbackReq{
		MerchantID: op.MerchantCode,
		UserID:     op.UserID,
		RoundID:    op.RoundID,
		Amount:     money.AmountFromMinor(op.BetAmount),
		Reason:     op.OpType,
	})
	if err != nil {
		if incErr := r.svcCtx.PendingOps.IncrementRetry(ctx, op.Id, err.Error()); incErr != nil {
			logx.Errorf("increment retry: id=%d err=%v", op.Id, incErr)
		}
		return
	}

	// 撤单回滚同样要在账本上留痕：这笔下注被退回了。
	// wallet.Rollback 不返回余额，因此钱包快照留空，balance_after 用本地推算值。
	postLedger(ctx, r.svcCtx, ledgerPosting{
		merchantID:   op.MerchantID,
		merchantCode: op.MerchantCode,
		userID:       op.UserID,
		roundID:      op.RoundID,
		typeCode:     model.TxTypeRollback,
		amount:       op.BetAmount,
		remark:       "撤单回滚退款",
		refType:      "wallet_pending_op",
		refID:        op.OpType,
	})

	if err := r.svcCtx.PendingOps.MarkDone(ctx, op.Id); err != nil {
		logx.Errorf("mark done: id=%d err=%v", op.Id, err)
	}
	logx.Infof("reconciled pending op: roundId=%s type=%s", op.RoundID, op.OpType)
}

// ledgerPosting 是补偿入账的参数包，避免每个调用点重复拼一长串字段。
type ledgerPosting struct {
	merchantID    uint64
	merchantCode  string
	userID        string
	roundID       string
	gameCode      string
	typeCode      string
	amount        int64 // minor units
	walletBalance *money.Amount
	remark        string
	refType       string
	refID         string
}

// postLedger 记一笔补偿账变（包级函数，两个对账循环共用）。
//
// 记账失败不阻断补偿流程（钱已经退回/付出去了），但必须打 error：
// 账本缺一条是资金事实的缺失，运维要能看见并补。
func postLedger(ctx context.Context, svcCtx *svc.ServiceContext, in ledgerPosting) {
	if svcCtx == nil || svcCtx.Ledger == nil || in.merchantID == 0 || in.amount <= 0 {
		return
	}
	res, err := svcCtx.Ledger.Post(ctx, ledger.Posting{
		TypeCode:      in.typeCode,
		MerchantID:    in.merchantID,
		MerchantCode:  in.merchantCode,
		UserID:        in.userID,
		GameCode:      in.gameCode,
		RoundID:       in.roundID,
		Amount:        money.AmountFromMinor(in.amount),
		WalletBalance: in.walletBalance,
		Status:        model.LedgerStatusSuccess,
		RefType:       in.refType,
		RefID:         in.refID,
		Remark:        in.remark,
	})
	if err != nil {
		logx.Errorf("补记账变失败: roundId=%s type=%s amount=%d err=%v",
			in.roundID, in.typeCode, in.amount, err)
		return
	}
	if res.Drift.Minor() != 0 {
		logx.Errorf("补记账变发现差额: roundId=%s type=%s drift=%d（本地镜像与钱包不一致，需核查）",
			in.roundID, in.typeCode, res.Drift.Minor())
	}
}
