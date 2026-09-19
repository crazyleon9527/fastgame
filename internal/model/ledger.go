package model

// ledger.go —— 账变体系的手写模型（迁移 35）
//
// 为什么手写而不是用 goctl 生成：账变流水是只追加的事实表，写路径需要
// 精确控制（悲观锁顺序、幂等 insert、事务内可见性），goctl 生成的
// 缓存式 CRUD（CachedConn + FindOne/Update/Delete）在这里既不合适也危险
// ——带缓存的余额读会直接毁掉账本一致性。
//
// 三张表的分工：
//   transaction_types  变动方向是数据（INCREASE/DECREASE/NONE），调用方只传正数金额
//   game_transactions  账变流水，每笔带前后余额快照 + 不可解释差额
//   player_accounts    影子账户，balance 是钱包返回值的镜像（钱包仍是权威）

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 账变终态
const (
	LedgerStatusSuccess      = "SUCCESS"       // 成功落账
	LedgerStatusFailed       = "FAILED"        // 钱包明确失败
	LedgerStatusPendingRetry = "PENDING_RETRY" // 待补偿重试（余额不动，由补偿链路收尾）
)

// 资金流向（冗余存储，便于按方向出账）
const (
	LedgerDirectionIn  = "IN"
	LedgerDirectionOut = "OUT"
)

// 余额变动方向（与 transaction_types.balance_change / frozen_change 取值一致）
const (
	ChangeIncrease = "INCREASE"
	ChangeDecrease = "DECREASE"
	ChangeNone     = "NONE"
)

// 账变类型编码。集中在这里，避免调用点各写一份字符串字面量——
// 写错一个字母就会在运行期被熔断（这是有意的：宁可拒绝也不能记错账）。
const (
	TxTypeBet         = "BET"
	TxTypeWin         = "WIN"
	TxTypeRefund      = "REFUND"
	TxTypeRollback    = "ROLLBACK"
	TxTypePromoCredit = "PROMO_CREDIT"
	TxTypeAdjustAdd   = "ADJUST_ADD"
	TxTypeAdjustSub   = "ADJUST_SUB"
)

// ErrTransactionTypeNotFound 账变类型不存在或已停用。
// 调用方应把它当配置错误处理并熔断，绝不能"猜一个方向"继续记账。
var ErrTransactionTypeNotFound = errors.New("账变类型不存在或已停用")

// TransactionType 对应表 transaction_types
type TransactionType struct {
	Id            uint64         `db:"id"`
	Code          string         `db:"code"`
	Scope         string         `db:"scope"`
	IoType        string         `db:"io_type"`
	BalanceChange string         `db:"balance_change"`
	FrozenChange  string         `db:"frozen_change"`
	Name          string         `db:"name"`
	NameI18n      sql.NullString `db:"name_i18n"`
	Description   sql.NullString `db:"description"`
	Status        int64          `db:"status"`
}

// GameTransaction 对应表 game_transactions
type GameTransaction struct {
	Id            uint64         `db:"id"`
	TransactionId string         `db:"transaction_id"`
	ExternalTxId  string         `db:"external_tx_id"`
	TypeId        uint64         `db:"type_id"`
	TxType        string         `db:"tx_type"`
	Direction     string         `db:"direction"`
	MerchantId    uint64         `db:"merchant_id"`
	MerchantCode  string         `db:"merchant_code"`
	UserId        string         `db:"user_id"`
	GameId        uint64         `db:"game_id"`
	GameCode      string         `db:"game_code"`
	RoundId       sql.NullString `db:"round_id"`
	Currency      string         `db:"currency"`
	Amount        int64          `db:"amount"`
	BalanceBefore int64          `db:"balance_before"`
	BalanceAfter  int64          `db:"balance_after"`
	FrozenBefore  int64          `db:"frozen_before"`
	FrozenAfter   int64          `db:"frozen_after"`
	DriftMinor    int64          `db:"drift_minor"`
	IsDemo        int64          `db:"is_demo"`
	Status        string         `db:"status"`
	Remark        string         `db:"remark"`
	ExtraData     sql.NullString `db:"extra_data"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

// SignedAmount 返回带符号的金额：进账为正、出账为负。
// 符号来自类型表的方向，而不是调用点传入——这是账变体系最关键的一条约定。
func (e *GameTransaction) SignedAmount() int64 {
	if e.Direction == LedgerDirectionIn {
		return e.Amount
	}
	return -e.Amount
}

// PlayerAccount 对应表 player_accounts（钱包余额的本地镜像）
type PlayerAccount struct {
	Id             uint64       `db:"id"`
	MerchantId     uint64       `db:"merchant_id"`
	MerchantCode   string       `db:"merchant_code"`
	UserId         string       `db:"user_id"`
	Currency       string       `db:"currency"`
	BalanceMinor   int64        `db:"balance_minor"`
	FrozenMinor    int64        `db:"frozen_minor"`
	Version        uint64       `db:"version"`
	LastLedgerId   uint64       `db:"last_ledger_id"`
	DriftCount     uint64       `db:"drift_count"`
	LastDriftMinor int64        `db:"last_drift_minor"`
	LastDriftAt    sql.NullTime `db:"last_drift_at"`
	CreatedAt      time.Time    `db:"created_at"`
	UpdatedAt      time.Time    `db:"updated_at"`
}

const gameTransactionColumns = `id, transaction_id, external_tx_id, type_id, tx_type, direction,
	merchant_id, merchant_code, user_id, game_id, game_code, round_id, currency,
	amount, balance_before, balance_after, frozen_before, frozen_after,
	drift_minor, is_demo, status, remark, extra_data, created_at, updated_at`

const playerAccountColumns = `id, merchant_id, merchant_code, user_id, currency,
	balance_minor, frozen_minor, version, last_ledger_id,
	drift_count, last_drift_minor, last_drift_at, created_at, updated_at`

// LedgerModel 账变体系的读写入口。
//
// 所有方法都接收 sqlx.SqlConn：它既能是普通连接，也能是
// sqlx.NewSqlConnFromSession(session) 包装出来的"事务内连接"，
// 因此同一份代码既能独立执行，也能并入调用方已开启的事务——
// 账变必须能和业务写（对账标记、回放记录、outbox 事件）同事务提交。
type LedgerModel interface {
	FindEnabledTypeByCode(ctx context.Context, conn sqlx.SqlConn, code string) (*TransactionType, error)
	ListEnabledTypes(ctx context.Context, conn sqlx.SqlConn) ([]*TransactionType, error)
	EnsureAccountForUpdate(ctx context.Context, conn sqlx.SqlConn, merchantID uint64, merchantCode, userID, currency string) (*PlayerAccount, error)
	UpdateAccount(ctx context.Context, conn sqlx.SqlConn, acc *PlayerAccount) error
	InsertTransaction(ctx context.Context, conn sqlx.SqlConn, entry *GameTransaction) (bool, error)
	// FindTransactionByRoundType 必须带 status：幂等键是
	// (merchant_id, round_id, tx_type, status)，同一局的同一类型可以
	// 先记一次 PENDING_RETRY、再记一次 SUCCESS（补偿落账）。
	FindTransactionByRoundType(ctx context.Context, conn sqlx.SqlConn, merchantID uint64, roundID, txType, status string) (*GameTransaction, error)
	FindTransactionByTransactionID(ctx context.Context, conn sqlx.SqlConn, transactionID string) (*GameTransaction, error)
	ListTransactions(ctx context.Context, conn sqlx.SqlConn, f LedgerQuery) ([]*GameTransaction, int64, error)
}

// LedgerQuery 流水查询条件（后台用）
type LedgerQuery struct {
	MerchantId uint64
	UserId     string
	TxType     string
	RoundId    string
	Status     string
	DriftOnly  bool
	From       *time.Time
	To         *time.Time
	Page       int
	PageSize   int
}

type defaultLedgerModel struct{}

func NewLedgerModel() LedgerModel { return &defaultLedgerModel{} }

func (m *defaultLedgerModel) FindEnabledTypeByCode(ctx context.Context, conn sqlx.SqlConn, code string) (*TransactionType, error) {
	var row TransactionType
	err := conn.QueryRowCtx(ctx, &row,
		`select id, code, scope, io_type, balance_change, frozen_change, name, name_i18n, description, status
		 from transaction_types where code = ? and status = 1 limit 1`, code)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrTransactionTypeNotFound
	default:
		return nil, err
	}
}

func (m *defaultLedgerModel) ListEnabledTypes(ctx context.Context, conn sqlx.SqlConn) ([]*TransactionType, error) {
	var list []*TransactionType
	err := conn.QueryRowsCtx(ctx, &list,
		`select id, code, scope, io_type, balance_change, frozen_change, name, name_i18n, description, status
		 from transaction_types where status = 1 order by id asc`)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// EnsureAccountForUpdate 锁定账户行；账户不存在时先建出来再锁。
//
// 先 INSERT ... ON DUPLICATE KEY UPDATE id=id 再 SELECT ... FOR UPDATE，
// 是为了让"首次账变"与并发首次账变都在同一行锁上串行化——
// 若先 SELECT 判空再 INSERT，两个并发请求会同时认为账户不存在，
// 其中一个必然撞唯一键失败。
func (m *defaultLedgerModel) EnsureAccountForUpdate(ctx context.Context, conn sqlx.SqlConn, merchantID uint64, merchantCode, userID, currency string) (*PlayerAccount, error) {
	if currency == "" {
		currency = "USD"
	}
	if _, err := conn.ExecCtx(ctx,
		`insert into player_accounts (merchant_id, merchant_code, user_id, currency, balance_minor, frozen_minor)
		 values (?, ?, ?, ?, 0, 0)
		 on duplicate key update id = id`,
		merchantID, merchantCode, userID, currency); err != nil {
		return nil, fmt.Errorf("ensure player account: %w", err)
	}

	var acc PlayerAccount
	err := conn.QueryRowCtx(ctx, &acc,
		`select `+playerAccountColumns+`
		 from player_accounts where merchant_id = ? and user_id = ? and currency = ?
		 for update`, merchantID, userID, currency)
	if err != nil {
		return nil, fmt.Errorf("lock player account: %w", err)
	}
	return &acc, nil
}

// UpdateAccount 写回余额镜像。用 version 做二次兜底：
// 行锁已经保证串行，若这里还更新到 0 行，说明有人绕过了 EnsureAccountForUpdate
// 直接改余额，属于必须暴露的严重问题，因此返回错误而不是静默继续。
func (m *defaultLedgerModel) UpdateAccount(ctx context.Context, conn sqlx.SqlConn, acc *PlayerAccount) error {
	res, err := conn.ExecCtx(ctx,
		`update player_accounts
		 set balance_minor = ?, frozen_minor = ?, last_ledger_id = ?,
		     drift_count = ?, last_drift_minor = ?, last_drift_at = ?,
		     version = version + 1
		 where id = ? and version = ?`,
		acc.BalanceMinor, acc.FrozenMinor, acc.LastLedgerId,
		acc.DriftCount, acc.LastDriftMinor, acc.LastDriftAt,
		acc.Id, acc.Version)
	if err != nil {
		return fmt.Errorf("update player account: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("player account version conflict: id=%d version=%d", acc.Id, acc.Version)
	}
	return nil
}

// InsertTransaction 插入账变流水，返回是否真的插入了。
//
// 幂等靠唯一键 uk_merchant_round_type(merchant_id, round_id, tx_type)：
// 同一商户同一局同一类型重复投递时 RowsAffected = 0（on duplicate key update id = id
// 不改任何值），调用方据此跳过账户变动，从而不会重复扣钱。
func (m *defaultLedgerModel) InsertTransaction(ctx context.Context, conn sqlx.SqlConn, entry *GameTransaction) (bool, error) {
	var roundID any
	if entry.RoundId.Valid && entry.RoundId.String != "" {
		roundID = entry.RoundId.String
	}
	var extra any
	if entry.ExtraData.Valid && entry.ExtraData.String != "" {
		extra = entry.ExtraData.String
	}
	res, err := conn.ExecCtx(ctx,
		`insert into game_transactions
		   (transaction_id, external_tx_id, type_id, tx_type, direction,
		    merchant_id, merchant_code, user_id, game_id, game_code, round_id, currency,
		    amount, balance_before, balance_after, frozen_before, frozen_after,
		    drift_minor, is_demo, status, remark, extra_data)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 on duplicate key update id = id`,
		entry.TransactionId, entry.ExternalTxId, entry.TypeId, entry.TxType, entry.Direction,
		entry.MerchantId, entry.MerchantCode, entry.UserId, entry.GameId, entry.GameCode, roundID, entry.Currency,
		entry.Amount, entry.BalanceBefore, entry.BalanceAfter, entry.FrozenBefore, entry.FrozenAfter,
		entry.DriftMinor, entry.IsDemo, entry.Status, entry.Remark, extra)
	if err != nil {
		return false, fmt.Errorf("insert game transaction: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (m *defaultLedgerModel) FindTransactionByRoundType(ctx context.Context, conn sqlx.SqlConn, merchantID uint64, roundID, txType, status string) (*GameTransaction, error) {
	var row GameTransaction
	err := conn.QueryRowCtx(ctx, &row,
		`select `+gameTransactionColumns+`
		 from game_transactions where merchant_id = ? and round_id = ? and tx_type = ? and status = ? limit 1`,
		merchantID, roundID, txType, status)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultLedgerModel) FindTransactionByTransactionID(ctx context.Context, conn sqlx.SqlConn, transactionID string) (*GameTransaction, error) {
	var row GameTransaction
	err := conn.QueryRowCtx(ctx, &row,
		`select `+gameTransactionColumns+` from game_transactions where transaction_id = ? limit 1`, transactionID)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultLedgerModel) ListTransactions(ctx context.Context, conn sqlx.SqlConn, f LedgerQuery) ([]*GameTransaction, int64, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.PageSize > 500 {
		f.PageSize = 500
	}

	var (
		where []string
		args  []any
	)
	if f.MerchantId > 0 {
		where = append(where, "merchant_id = ?")
		args = append(args, f.MerchantId)
	}
	if f.UserId != "" {
		where = append(where, "user_id = ?")
		args = append(args, f.UserId)
	}
	if f.TxType != "" {
		where = append(where, "tx_type = ?")
		args = append(args, f.TxType)
	}
	if f.RoundId != "" {
		where = append(where, "round_id = ?")
		args = append(args, f.RoundId)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.DriftOnly {
		where = append(where, "drift_minor <> 0")
	}
	if f.From != nil {
		where = append(where, "created_at >= ?")
		args = append(args, *f.From)
	}
	if f.To != nil {
		where = append(where, "created_at <= ?")
		args = append(args, *f.To)
	}

	clause := ""
	if len(where) > 0 {
		clause = " where " + strings.Join(where, " and ")
	}

	var total int64
	if err := conn.QueryRowCtx(ctx, &total, "select count(*) from game_transactions"+clause, args...); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]any{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	var list []*GameTransaction
	if err := conn.QueryRowsCtx(ctx, &list,
		"select "+gameTransactionColumns+" from game_transactions"+clause+" order by id desc limit ? offset ?",
		listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
