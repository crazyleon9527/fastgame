package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/ledger"
	"fastgame/pkg/money"
	"fastgame/pkg/outbox"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 人工调账只允许这两种类型：加款与扣款。
//
// 白名单在后台本地再过一遍，目的是让"调错方向"在接口层就被挡住：
// Poster 的方向来自类型表（数据），一旦有人在类型表里把 ADJUST_ADD 配成
// DECREASE，仅靠 Poster 是拦不住的，这里至少保证后台只能走这两个 code。
const (
	adjustRefType = "admin_adjustment"
	// adjustReasonRequired 审计最低要求：没有原因的人工调账不允许落账。
	adjustReasonRequired = "人工调账必须填写调整原因（remark）"
)

// ErrAdjustTypeUnsupported 人工调账只接受 ADJUST_ADD / ADJUST_SUB。
// 后台把它映射成 400；这是调用方参数问题，不是服务端故障。
var ErrAdjustTypeUnsupported = errors.New("人工调账只支持 ADJUST_ADD（加款）或 ADJUST_SUB（扣款）")

type LedgerAdjustmentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLedgerAdjustmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LedgerAdjustmentLogic {
	return &LedgerAdjustmentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Adjust 登记一笔人工调账。
//
// 语义：这不是"在本地凭空改余额"，而是把**已经在商户钱包侧做过的人工调整**
// 登记进本地台账——钱包才是余额的权威，本地 player_accounts 只是镜像。
// 因此 externalTxId（钱包凭证号）与 remark（调整原因）必填；WalletBalance 传 nil，
// 表示本次操作钱包没有回余额，本地按类型方向推算镜像（drift_minor 记 0，
// 下一笔带钱包余额的账变会自然把差异暴露出来）。
func (l *LedgerAdjustmentLogic) Adjust(req *types.LedgerAdjustmentReq) (*types.LedgerAdjustmentResp, error) {
	typeCode := strings.TrimSpace(req.TypeCode)
	switch typeCode {
	case model.TxTypeAdjustAdd, model.TxTypeAdjustSub:
	default:
		return nil, fmt.Errorf("%w: %s", ErrAdjustTypeUnsupported, typeCode)
	}

	// go-zero 的 optional 标签不会把空串变成"未提供"，所以必填项在这里自己校验，
	// 保证错误信息是中文且能指出是哪一项。
	if req.MerchantId == 0 {
		return nil, errors.New("merchantId 必填")
	}
	if strings.TrimSpace(req.UserId) == "" {
		return nil, errors.New("userId 必填")
	}
	if req.AmountMinor <= 0 {
		// 提前拦住，避免走到 Poster 才报"金额必须为正数"，错误信息更贴近后台表单。
		return nil, ledger.ErrAmountNotPositive
	}
	if strings.TrimSpace(req.ExternalTxId) == "" {
		return nil, errors.New("externalTxId 必填：人工调账必须登记钱包侧凭证号，否则无法审计")
	}
	if strings.TrimSpace(req.Remark) == "" {
		return nil, errors.New(adjustReasonRequired)
	}

	operatorID, err := userIDFromCtx(l.ctx)
	if err != nil {
		return nil, err
	}
	// 操作人必须落进 extra_data：人工调账是"人"发起的高危动作，
	// 事后追责只能靠这里，不能依赖 HTTP 访问日志。
	extra := map[string]any{"operator_admin_id": operatorID}
	if username, err := l.operatorUsername(operatorID); err != nil {
		// 用户名取不到不阻断调账（id 已在），但必须留日志，避免审计信息静默缺失。
		l.Errorf("人工调账：查询操作人用户名失败 admin_id=%d: %v", operatorID, err)
	} else if username != "" {
		extra["operator_username"] = username
	}
	if req.OperatorIp != "" {
		extra["operator_ip"] = req.OperatorIp
	}

	// 流水号提前生成：这样即使幂等回查失败，返回体里的 transactionId 也不是空的。
	transactionID, err := outbox.NewEventID()
	if err != nil {
		return nil, fmt.Errorf("生成账变流水号失败: %w", err)
	}

	res, err := l.svcCtx.Ledger.Post(l.ctx, ledger.Posting{
		TypeCode:   typeCode,
		MerchantID: req.MerchantId,
		UserID:     strings.TrimSpace(req.UserId),
		// RoundID 留空：人工调账不属于任何一局。幂等键含 round_id，
		// MySQL 唯一索引视 NULL 互不相同，因此同一玩家可以多次人工调整。
		Amount:       money.AmountFromMinor(req.AmountMinor),
		Currency:     req.Currency,
		ExternalTxID: strings.TrimSpace(req.ExternalTxId),
		Status:       model.LedgerStatusSuccess,
		// WalletBalance 传 nil：钱包侧调整已经发生，但本次调用没有回余额快照，
		// 本地按类型方向推算即可（传了才会算 drift_minor）。
		WalletBalance: nil,
		TransactionID: transactionID,
		RefType:       adjustRefType,
		// RefID 用钱包凭证号：从流水能直接反查出钱包侧那笔调整。
		RefID:  strings.TrimSpace(req.ExternalTxId),
		Remark: strings.TrimSpace(req.Remark),
		Extra:  extra,
	})
	if err != nil {
		// 账变类型不可用属于配置问题，换成中文哨兵错误交给 handler 判 400；
		// 其余错误原样上抛（handler 视为 500），避免把服务端故障伪装成参数错误。
		if errors.Is(err, model.ErrTransactionTypeNotFound) {
			return nil, fmt.Errorf("%w: %s", model.ErrTransactionTypeNotFound, typeCode)
		}
		return nil, err
	}

	l.Infof("人工调账已登记: merchant=%d user=%s type=%s amount=%d external_tx=%s transaction=%s duplicated=%v",
		req.MerchantId, req.UserId, typeCode, req.AmountMinor,
		req.ExternalTxId, res.Entry.TransactionId, res.Duplicated)

	return &types.LedgerAdjustmentResp{
		TransactionId:      res.Entry.TransactionId,
		TxType:             res.Entry.TxType,
		Direction:          res.Entry.Direction,
		AmountMinor:        res.Entry.Amount,
		BalanceBeforeMinor: res.Entry.BalanceBefore,
		BalanceAfterMinor:  res.Entry.BalanceAfter,
		Status:             res.Entry.Status,
		DriftMinor:         res.Entry.DriftMinor,
		Duplicated:         res.Duplicated,
	}, nil
}

// operatorUsername 从 admin_users 取当前操作人的用户名。
//
// 之所以现查库而不是改中间件往上下文里塞：JWT 里只有 userId，加字段要动
// 登录与鉴权链路；这里一次主键查询就能拿到，属于人工调账这种低频接口可以接受的成本。
func (l *LedgerAdjustmentLogic) operatorUsername(adminID uint64) (string, error) {
	user, err := l.svcCtx.AdminUsers.FindOne(l.ctx, adminID)
	if err != nil {
		return "", err
	}
	return user.Username, nil
}

// parseLedgerTime 兼容三种时间写法：
//   - RFC3339（带时区，最明确）
//   - "2006-01-02 15:04:05"（后台表单常见写法，按 UTC 解释）
//   - "2006-01-02"（按当天 00:00:00 UTC 解释）
//
// 为什么默认 UTC 而不是本地时区：账变 created_at 一律以 UTC 入库，
// 用服务器本地时区解释会让"同一条查询在不同机器上结果不同"。
func parseLedgerTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("时间格式不正确（%s），支持 2006-01-02 15:04:05 / RFC3339", s)
}
