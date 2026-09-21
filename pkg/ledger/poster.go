package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/money"
	"fastgame/pkg/outbox"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 配置默认值
const (
	defaultCurrency = "USD"
	typeCacheTTL    = 30 * time.Second
	maxRemarkLen    = 255
)

// ErrAmountNotPositive 金额必须为正数
var ErrAmountNotPositive = errors.New("账变金额必须为正数")

// ErrInsufficientBalance 余额不足
var ErrInsufficientBalance = errors.New("余额不足，账变被拒绝")

// Posting 一次账变请求
type Posting struct {
	TypeCode     string // 必填：transaction_types.code
	MerchantID   uint64 // 必填
	MerchantCode string
	UserID       string // 必填
	GameID       uint64
	GameCode     string
	RoundID      string // 无单局账变（人工调账）留空
	Amount       money.Amount
	Currency     string
	IsDemo       bool

	// WalletBalance 钱包返回的余额快照
	WalletBalance *money.Amount
	// ExternalTxID 下游钱包返回的三方对账凭据号
	ExternalTxID string
	// Status 账变终态
	Status string

	// TransactionID 可留空（自动生成）
	TransactionID string
	// RefType / RefID 关联业务对象
	RefType string
	RefID   string
	Remark  string
	Extra   map[string]any
}

// Result 一次账变的结果
type Result struct {
	Entry      *model.GameTransaction
	Account    *model.PlayerAccount
	Type       *model.TransactionType
	Drift      money.Amount // 不可解释差额，0 表示本地流水与钱包一致
	Duplicated bool         // true = 命中幂等键，本次没有重复扣钱
}

// Sink 账变事件的出口
type Sink interface {
	PublishInTx(ctx context.Context, session sqlx.Session, entry *model.GameTransaction) error
}

// Clock 便于测试注入时间
type Clock func() time.Time

// Poster 账变收口
type Poster struct {
	conn  sqlx.SqlConn
	model model.LedgerModel
	sink  Sink
	now   Clock

	typeMu  sync.RWMutex
	types   map[string]*model.TransactionType
	typesAt time.Time
}

// NewPoster 构造收口
func NewPoster(conn sqlx.SqlConn, sink Sink) *Poster {
	return &Poster{
		conn:  conn,
		model: model.NewLedgerModel(),
		sink:  sink,
		now:   func() time.Time { return time.Now().UTC() },
		types: make(map[string]*model.TransactionType),
	}
}

// Post 自动开事务地记一笔账变
func (p *Poster) Post(ctx context.Context, in Posting) (*Result, error) {
	var res *Result
	err := p.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		res, err = p.PostInTx(ctx, sqlx.NewSqlConnFromSession(session), session, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// PostInTx 在调用方事务内记一笔账变
func (p *Poster) PostInTx(ctx context.Context, conn sqlx.SqlConn, session sqlx.Session, in Posting) (*Result, error) {
	if in.Amount <= 0 {
		return nil, ErrAmountNotPositive
	}
	if in.UserID == "" {
		return nil, fmt.Errorf("账变缺少 user_id")
	}

	txType, err := p.typeByCode(ctx, conn, in.TypeCode)
	if err != nil {
		return nil, err
	}

	currency := in.Currency
	if currency == "" {
		currency = defaultCurrency
	}
	status := in.Status
	if status == "" {
		status = model.LedgerStatusSuccess
	}

	acc, err := p.model.EnsureAccountForUpdate(ctx, conn, in.MerchantID, in.MerchantCode, in.UserID, currency)
	if err != nil {
		return nil, err
	}

	entry := &model.GameTransaction{
		TransactionId: in.TransactionID,
		ExternalTxId:  in.ExternalTxID,
		TypeId:        txType.Id,
		TxType:        txType.Code,
		Direction:     directionOf(txType.IoType),
		MerchantId:    in.MerchantID,
		MerchantCode:  in.MerchantCode,
		UserId:        in.UserID,
		GameId:        in.GameID,
		GameCode:      in.GameCode,
		Currency:      currency,
		Amount:        in.Amount.Minor(),
		BalanceBefore: acc.BalanceMinor,
		BalanceAfter:  acc.BalanceMinor,
		FrozenBefore:  acc.FrozenMinor,
		FrozenAfter:   acc.FrozenMinor,
		IsDemo:        boolToTiny(in.IsDemo),
		Status:        status,
		Remark:        truncate(in.Remark, maxRemarkLen),
	}
	if in.RoundID != "" {
		entry.RoundId = sql.NullString{String: in.RoundID, Valid: true}
	}
	if extra := encodeExtra(in); extra != "" {
		entry.ExtraData = sql.NullString{String: extra, Valid: true}
	}
	if entry.TransactionId == "" {
		id, err := outbox.NewEventID()
		if err != nil {
			return nil, fmt.Errorf("生成账变流水号失败: %w", err)
		}
		entry.TransactionId = id
	}

	// 终态不是 SUCCESS 时钱没动：只留痕，余额镜像保持原值
	if status != model.LedgerStatusSuccess {
		inserted, err := p.model.InsertTransaction(ctx, conn, entry)
		if err != nil {
			return nil, err
		}
		if !inserted {
			return p.duplicated(ctx, conn, in)
		}
		return &Result{Entry: entry, Account: acc, Type: txType}, nil
	}

	// 按类型推算新余额
	enforceBalance := in.WalletBalance == nil
	computed, frozenAfter, err := applyChange(acc, txType, in.Amount.Minor(), enforceBalance)
	if err != nil {
		return nil, err
	}
	entry.FrozenAfter = frozenAfter

	// 钱包返回余额时以钱包为准
	after := computed
	if in.WalletBalance != nil {
		after = in.WalletBalance.Minor()
		entry.DriftMinor = after - computed
	}
	entry.BalanceAfter = after

	inserted, err := p.model.InsertTransaction(ctx, conn, entry)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return p.duplicated(ctx, conn, in)
	}

	acc.BalanceMinor = after
	acc.FrozenMinor = frozenAfter
	if entry.DriftMinor != 0 {
		acc.DriftCount++
		acc.LastDriftMinor = entry.DriftMinor
		acc.LastDriftAt = sql.NullTime{Time: p.now(), Valid: true}
		logx.WithContext(ctx).Errorw("ledger_unexplained_drift",
			logx.Field("merchant_id", in.MerchantID),
			logx.Field("user_id", in.UserID),
			logx.Field("round_id", in.RoundID),
			logx.Field("tx_type", txType.Code),
			logx.Field("wallet_balance", after),
			logx.Field("computed_balance", computed),
			logx.Field("drift", entry.DriftMinor),
		)
	}

	// 回查流水 ID
	if saved, err := p.model.FindTransactionByTransactionID(ctx, conn, entry.TransactionId); err == nil && saved != nil {
		entry.Id = saved.Id
	}
	acc.LastLedgerId = entry.Id

	if err := p.model.UpdateAccount(ctx, conn, acc); err != nil {
		return nil, err
	}

	if p.sink != nil && session != nil {
		if err := p.sink.PublishInTx(ctx, session, entry); err != nil {
			return nil, fmt.Errorf("登记账变事件失败: %w", err)
		}
	}

	return &Result{Entry: entry, Account: acc, Type: txType, Drift: money.AmountFromMinor(entry.DriftMinor)}, nil
}

func (p *Poster) duplicated(ctx context.Context, conn sqlx.SqlConn, in Posting) (*Result, error) {
	status := in.Status
	if status == "" {
		status = model.LedgerStatusSuccess
	}
	var existing *model.GameTransaction
	if in.RoundID != "" {
		existing, _ = p.model.FindTransactionByRoundType(ctx, conn, in.MerchantID, in.RoundID, in.TypeCode, status)
	}
	if existing == nil && in.TransactionID != "" {
		if byID, err := p.model.FindTransactionByTransactionID(ctx, conn, in.TransactionID); err == nil {
			existing = byID
		}
	}
	if existing == nil {
		return nil, fmt.Errorf("账变命中幂等键但回查不到原记录: merchant=%d round=%s type=%s status=%s",
			in.MerchantID, in.RoundID, in.TypeCode, status)
	}
	logx.WithContext(ctx).Infow("ledger_duplicate_ignored",
		logx.Field("merchant_id", in.MerchantID),
		logx.Field("round_id", in.RoundID),
		logx.Field("tx_type", in.TypeCode),
		logx.Field("transaction_id", existing.TransactionId),
	)
	return &Result{Entry: existing, Duplicated: true}, nil
}

// typeByCode 带线程安全读写锁的类型查询
func (p *Poster) typeByCode(ctx context.Context, conn sqlx.SqlConn, code string) (*model.TransactionType, error) {
	if code == "" {
		return nil, model.ErrTransactionTypeNotFound
	}

	now := p.now()

	// 1. 尝试读锁获取缓存
	p.typeMu.RLock()
	if now.Sub(p.typesAt) < typeCacheTTL && len(p.types) > 0 {
		t, ok := p.types[code]
		p.typeMu.RUnlock()
		if ok {
			return t, nil
		}
	} else {
		p.typeMu.RUnlock()
	}

	// 2. 加写锁更新缓存
	p.typeMu.Lock()
	defer p.typeMu.Unlock()

	// Double-Check 避免并发重读
	if now.Sub(p.typesAt) < typeCacheTTL && len(p.types) > 0 {
		if t, ok := p.types[code]; ok {
			return t, nil
		}
	}

	list, err := p.model.ListEnabledTypes(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("加载账变类型失败: %w", err)
	}
	fresh := make(map[string]*model.TransactionType, len(list))
	for _, t := range list {
		fresh[t.Code] = t
	}
	p.types = fresh
	p.typesAt = now

	t, ok := fresh[code]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrTransactionTypeNotFound, code)
	}
	return t, nil
}

// InvalidateTypes 清掉类型缓存
func (p *Poster) InvalidateTypes() {
	p.typeMu.Lock()
	defer p.typeMu.Unlock()
	p.types = make(map[string]*model.TransactionType)
	p.typesAt = time.Time{}
}

func directionOf(ioType string) string {
	if ioType == "OUT" {
		return model.LedgerDirectionOut
	}
	return model.LedgerDirectionIn
}

func applyChange(acc *model.PlayerAccount, t *model.TransactionType, amount int64, enforceBalance bool) (balance, frozen int64, err error) {
	balance, frozen = acc.BalanceMinor, acc.FrozenMinor

	switch t.BalanceChange {
	case model.ChangeIncrease:
		balance += amount
	case model.ChangeDecrease:
		if enforceBalance && balance < amount {
			return 0, 0, fmt.Errorf("%w: 可用余额 %d, 需要 %d (type=%s)", ErrInsufficientBalance, balance, amount, t.Code)
		}
		balance -= amount
	case model.ChangeNone:
	default:
		return 0, 0, fmt.Errorf("账变类型 %s 的 balance_change 配置非法: %q", t.Code, t.BalanceChange)
	}

	switch t.FrozenChange {
	case model.ChangeIncrease:
		frozen += amount
	case model.ChangeDecrease:
		if enforceBalance && frozen < amount {
			return 0, 0, fmt.Errorf("%w: 冻结余额 %d, 需要 %d (type=%s)", ErrInsufficientBalance, frozen, amount, t.Code)
		}
		frozen -= amount
	case model.ChangeNone:
	default:
		return 0, 0, fmt.Errorf("账变类型 %s 的 frozen_change 配置非法: %q", t.Code, t.FrozenChange)
	}

	if enforceBalance && (balance < 0 || frozen < 0) {
		return 0, 0, fmt.Errorf("%w: 计算后余额为负 (type=%s)", ErrInsufficientBalance, t.Code)
	}
	return balance, frozen, nil
}

func encodeExtra(in Posting) string {
	extra := map[string]any{}
	for k, v := range in.Extra {
		extra[k] = v
	}
	if in.RefType != "" {
		extra["ref_type"] = in.RefType
	}
	if in.RefID != "" {
		extra["ref_id"] = in.RefID
	}
	if len(extra) == 0 {
		return ""
	}
	b, err := json.Marshal(extra)
	if err != nil {
		return ""
	}
	return string(b)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func boolToTiny(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
