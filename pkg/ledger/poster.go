// Package ledger 是账变的唯一收口。
//
// 参考 platform-api 的 CashService.AddTransaction / AddTransactionInTx，核心约定：
//
//  1. 变动方向是数据，不是代码。调用方只传 TypeCode + **正数**金额，
//     由 transaction_types 决定加减可用余额还是冻结余额。调用点因此不可能
//     写错符号，新增业务类型只要插一行数据。
//  2. 余额快照随账变落库（balance_before/after）。对账不必从头累加，
//     任意时间点都能核对。
//  3. 只有一个写入口。任何绕过 Poster 直接改余额的代码，都会在下一次
//     连续性校验里暴露成"不可解释差额"。
//
// 与 platform-api 的差异，也是 fastgame 的架构事实：
//
//	玩家余额的权威在**商户钱包**（外部服务）侧，fastgame 不拥有它。
//	因此这里实现的是"影子账"——player_accounts 里的余额是钱包返回值的镜像：
//	  · 钱包返回余额时，以钱包为准写入 balance_after；
//	    若与本地按类型推算的结果不一致，把差额记进 drift_minor（不可解释差额）；
//	  · 钱包没返回余额时（例如撤单回滚接口不回余额），用本地推算值续上。
//	这样既不动资金主链路，又能立刻得到「可对账的流水 + 差异发现」能力。
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/money"
	"fastgame/pkg/outbox"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 配置默认值
const (
	// defaultCurrency 玩家钱包的结算币种。fastgame 目前是单币种，
	// 多币种接入时改为从商户配置读取。
	defaultCurrency = "USD"
	// typeCacheTTL 账变类型的内存缓存时长。类型表几乎不变，
	// 但改了要能较快生效（例如紧急停用某个类型）。
	typeCacheTTL = 30 * time.Second
	// maxRemarkLen 与 game_transactions.remark 的列宽一致（varchar(255)）。
	maxRemarkLen = 255
)

// ErrAmountNotPositive 金额必须为正数：方向由类型表决定，
// 传负数会让"加减"和"方向"两处同时表达符号，必然出错。
var ErrAmountNotPositive = errors.New("账变金额必须为正数")

// ErrInsufficientBalance 余额不足（按类型推算后为负）。
// 影子账下它不代表钱包真的拒绝，只说明本地镜像与请求矛盾——
// 属于必须人工核查的异常，因此拒绝写入而不是记成 0。
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

	// WalletBalance 是钱包返回的余额快照。传 nil 表示本次操作钱包没回余额，
	// 此时 balance_after 用本地推算值（镜像自洽）。
	WalletBalance *money.Amount
	// ExternalTxID 下游钱包返回的三方对账凭据号。
	ExternalTxID string
	// Status 账变终态。只有 SUCCESS 会改动余额镜像：
	// FAILED / PENDING_RETRY 只留痕（钱没动，由补偿链路收尾）。
	Status string

	// TransactionID 可留空（自动生成 UUID v7）。需要调用方自带幂等键时传入。
	TransactionID string
	// RefType / RefID 关联业务对象，便于从流水反查来源。
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

// Sink 账变事件的出口。
//
// 做成接口而不是直接依赖 outbox：只有已经在跑 outbox dispatcher 的服务
// （当前是 RGS）能把事件真正投递出去。其他服务先传 nil，
// 待接入结算/对账时再补 dispatcher——账变本身的持久化不依赖它。
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

	types     map[string]*model.TransactionType
	typesAt   time.Time
	typesLoad bool
}

// NewPoster 构造收口。conn 用于「自动开事务」的 Post；
// sink 可为 nil（不投递账变事件）。
func NewPoster(conn sqlx.SqlConn, sink Sink) *Poster {
	return &Poster{
		conn:  conn,
		model: model.NewLedgerModel(),
		sink:  sink,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// Post 自动开事务地记一笔账变。
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

// PostInTx 在调用方事务内记一笔账变。
//
// 顺序很重要：先锁账户 → 再插流水（幂等判定）→ 最后改余额。
// 反过来先改余额再插流水，一旦流水插入失败（撞幂等键）余额就多扣了一次。
//
// conn 是事务内连接（用于 SQL），session 是同一条事务（用于 outbox 事件）。
// 不需要投递事件时 session 可以传 nil。
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

	// 终态不是 SUCCESS 时钱没动：只留痕，余额镜像保持原值。
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

	// 按类型推算新余额（方向来自类型表）
	//
	// enforceBalance 的取值很关键：只有"本地没有权威余额、也没拿到钱包余额"时
	// 才拒绝负数。钱包返回了余额就说明钱确实动过，此时本地推算出负数
	// 只能说明**镜像偏了**（例如玩家首次出现、镜像还没建立），
	// 应当以钱包为准并把差额记成 drift，而不是拒绝记账——
	// 拒绝的后果是这笔真实资金变动在账本上完全消失。
	enforceBalance := in.WalletBalance == nil
	computed, frozenAfter, err := applyChange(acc, txType, in.Amount.Minor(), enforceBalance)
	if err != nil {
		return nil, err
	}
	entry.FrozenAfter = frozenAfter

	// 钱包返回余额时以钱包为准，并检查是否与推算一致
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
		// 幂等命中：本条已记过，绝不能再改一次余额
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

	// 先取回流水主键再写账户：last_ledger_id 要指向本条流水。
	// INSERT 之后驱动不一定回填 entry.Id，所以按唯一键回查；
	// 回查失败不阻断（余额与流水已经一致，只影响 last_ledger_id 的精度）。
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

// duplicated 处理幂等命中：返回已存在的那条流水，且**不做任何余额变动**。
// 幂等键含终态，因此 FAILED/PENDING_RETRY 与 SUCCESS 各自只记一次、互不干扰。
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

// typeByCode 取账变类型（带进程内缓存）。
//
// 取不到就报错熔断：绝不"猜一个方向"继续记账——方向猜错就是记反账，
// 比拒绝一次请求严重得多。
func (p *Poster) typeByCode(ctx context.Context, conn sqlx.SqlConn, code string) (*model.TransactionType, error) {
	if code == "" {
		return nil, model.ErrTransactionTypeNotFound
	}
	if p.typesLoad && p.now().Sub(p.typesAt) < typeCacheTTL {
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
	p.types, p.typesAt, p.typesLoad = fresh, p.now(), true

	t, ok := fresh[code]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrTransactionTypeNotFound, code)
	}
	return t, nil
}

// InvalidateTypes 清掉类型缓存（后台改了类型表后调用）。
func (p *Poster) InvalidateTypes() {
	p.typesLoad = false
	p.types = nil
}

// directionOf 把 io_type 映射成流水里的资金流向冗余字段。
func directionOf(ioType string) string {
	if ioType == "OUT" {
		return model.LedgerDirectionOut
	}
	return model.LedgerDirectionIn
}

// applyChange 按类型表的方向推算变动后的余额。
//
// enforceBalance=true 时余额不足/算完为负直接拒绝；false 时只做算术，
// 把"算出来是负数"留给 drift 去表达（钱包返回了余额，钱确实动过，
// 本地负数只说明镜像偏了）。
//
// 未知的变动配置一律报错（熔断），而不是当成 NONE 放行：
// 配置写错时"静默不生效"会让账实不符且无人察觉。
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

// encodeExtra 组装 extra_data。除调用方给的键值外，固定写入幂等键与备注来源，
// 便于事后只靠一行流水还原"这笔钱是怎么来的"。
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

// truncate 按 **rune** 截断，与 MySQL varchar(255) 的字符语义一致。
// 按字节截断会把多字节 UTF-8 字符切坏，remark 里出现乱码。
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
