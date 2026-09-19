package ledger

// poster_test.go —— 账变收口的行为测试（需要 fastgame-mysql 容器在线）
//
// 这里断言的不是"能跑通"，而是账本必须成立的那几条不变量：
//   · 方向由类型表决定，调用方传正数（写错符号不可能发生）
//   · 同一局同一类型重复投递不会重复扣钱（幂等）
//   · 余额不足被拒绝，而不是记成负数或记成 0
//   · 未知/停用的类型直接熔断，不"猜方向"
//   · 钱包返回余额时以钱包为准，差额记进 drift_minor（不可解释差额）
//   · 失败/待补偿状态只留痕、不动余额
//   · 前后快照连续（本笔 before == 上笔 after）
//
// MySQL 不可达时自动跳过。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"fastgame/internal/model"
	"fastgame/pkg/money"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const defaultDSN = "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"

func ledgerDSN() string {
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		return v
	}
	return defaultDSN
}

// ledgerProbe 一个探针上下文：真实 Poster + 真实库，测试结束清掉自己的数据。
type ledgerProbe struct {
	t      *testing.T
	conn   sqlx.SqlConn
	db     *sql.DB
	poster *Poster
	mc     uint64
	user   string
}

func newLedgerProbe(t *testing.T) *ledgerProbe {
	t.Helper()
	dsn := ledgerDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("跳过：无法创建 MySQL 连接（%v）", err)
	}
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("跳过：MySQL 不可达（%v）。先执行 docker compose up -d mysql", err)
	}
	conn := sqlx.NewMysql(dsn)

	// 探针用远离种子数据的商户 ID，避免与业务数据纠缠
	p := &ledgerProbe{
		t:    t,
		conn: conn,
		db:   db,
		mc:   990001,
		user: "zz-ledger-probe-user",
	}
	p.poster = NewPoster(conn, nil)
	p.cleanup()
	t.Cleanup(func() {
		p.cleanup()
		db.Close()
	})
	return p
}

func (p *ledgerProbe) cleanup() {
	p.db.Exec("DELETE FROM game_transactions WHERE merchant_id = ?", p.mc)
	p.db.Exec("DELETE FROM player_accounts WHERE merchant_id = ?", p.mc)
}

func (p *ledgerProbe) post(in Posting) (*Result, error) {
	p.t.Helper()
	if in.MerchantID == 0 {
		in.MerchantID = p.mc
	}
	if in.MerchantCode == "" {
		in.MerchantCode = "zz-ledger"
	}
	if in.UserID == "" {
		in.UserID = p.user
	}
	return p.poster.Post(context.Background(), in)
}

func (p *ledgerProbe) balance() int64 {
	p.t.Helper()
	var v int64
	if err := p.db.QueryRow("SELECT balance_minor FROM player_accounts WHERE merchant_id = ? AND user_id = ?",
		p.mc, p.user).Scan(&v); err != nil {
		p.t.Fatalf("读取账户余额失败: %v", err)
	}
	return v
}

// TestLedgerDirectionComesFromTypeTable —— 核心约定：
// 调用方只传正数金额，BET 必须是"扣"、WIN 必须是"加"。
func TestLedgerDirectionComesFromTypeTable(t *testing.T) {
	p := newLedgerProbe(t)
	bet := money.FromMajor(100)

	// 先给玩家一笔加款，作为下注的本金
	if _, err := p.post(Posting{TypeCode: model.TxTypeAdjustAdd, Amount: bet, Remark: "探针初始余额",
		WalletBalance: ptr(money.FromMajor(100))}); err != nil {
		t.Fatalf("ADJUST_ADD 失败: %v", err)
	}
	if got := p.balance(); got != bet.Minor() {
		t.Fatalf("加款后余额 = %d，期望 %d", got, bet.Minor())
	}

	// BET = OUT/DECREASE
	res, err := p.post(Posting{TypeCode: model.TxTypeBet, Amount: bet, RoundID: "zz-dir-1",
		WalletBalance: ptr(money.Amount(0))})
	if err != nil {
		t.Fatalf("BET 失败: %v", err)
	}
	if res.Entry.Direction != model.LedgerDirectionOut {
		t.Errorf("BET 方向 = %s，期望 %s（应来自 transaction_types.io_type）",
			res.Entry.Direction, model.LedgerDirectionOut)
	}
	if got := p.balance(); got != 0 {
		t.Fatalf("下注后余额 = %d，期望 0 —— 方向没有按类型表生效", got)
	}
	if res.Entry.BalanceBefore != bet.Minor() || res.Entry.BalanceAfter != 0 {
		t.Errorf("前后快照 = (%d, %d)，期望 (%d, 0)",
			res.Entry.BalanceBefore, res.Entry.BalanceAfter, bet.Minor())
	}

	// WIN = IN/INCREASE
	win := money.FromMajor(250)
	res2, err := p.post(Posting{TypeCode: model.TxTypeWin, Amount: win, RoundID: "zz-dir-1",
		WalletBalance: ptr(win)})
	if err != nil {
		t.Fatalf("WIN 失败: %v", err)
	}
	if res2.Entry.Direction != model.LedgerDirectionIn {
		t.Errorf("WIN 方向 = %s，期望 %s", res2.Entry.Direction, model.LedgerDirectionIn)
	}
	if got := p.balance(); got != win.Minor() {
		t.Fatalf("派彩后余额 = %d，期望 %d", got, win.Minor())
	}
}

// TestLedgerIdempotentPerRoundType —— 重复投递同一局同一类型不能重复扣钱。
func TestLedgerIdempotentPerRoundType(t *testing.T) {
	p := newLedgerProbe(t)
	fund := money.FromMajor(100)
	if _, err := p.post(Posting{TypeCode: model.TxTypeAdjustAdd, Amount: fund,
		WalletBalance: ptr(fund)}); err != nil {
		t.Fatal(err)
	}

	bet := money.FromMajor(30)
	first, err := p.post(Posting{TypeCode: model.TxTypeBet, Amount: bet, RoundID: "zz-idem-1",
		WalletBalance: ptr(money.FromMajor(70))})
	if err != nil {
		t.Fatalf("首次 BET 失败: %v", err)
	}
	if first.Duplicated {
		t.Fatal("首次投递不应被判为重复")
	}

	// 同一个 round + 同一个类型再投一次（模拟重复消费/重放）
	second, err := p.post(Posting{TypeCode: model.TxTypeBet, Amount: bet, RoundID: "zz-idem-1",
		WalletBalance: ptr(money.FromMajor(70))})
	if err != nil {
		t.Fatalf("重放 BET 返回错误（应当幂等成功）: %v", err)
	}
	if !second.Duplicated {
		t.Fatal("第二次投递应命中幂等键")
	}
	if second.Entry.TransactionId != first.Entry.TransactionId {
		t.Errorf("幂等命中却返回了不同流水: %s vs %s", second.Entry.TransactionId, first.Entry.TransactionId)
	}
	if got, want := p.balance(), money.FromMajor(70).Minor(); got != want {
		t.Fatalf("余额 = %d，期望 %d —— 重放导致重复扣钱", got, want)
	}

	var cnt int64
	if err := p.db.QueryRow("SELECT COUNT(*) FROM game_transactions WHERE merchant_id = ? AND round_id = ?",
		p.mc, "zz-idem-1").Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("同局同类型流水 = %d 条，期望 1 条", cnt)
	}
}

// TestLedgerRejectsInsufficientBalance —— 本地没有权威余额、也没拿到钱包余额时
// 必须拒绝，不能凭空记出负数。
func TestLedgerRejectsInsufficientBalance(t *testing.T) {
	p := newLedgerProbe(t)
	// WalletBalance 传 nil：既没有钱包权威值，镜像也是 0 → 必须拒绝
	_, err := p.post(Posting{TypeCode: model.TxTypeBet, Amount: money.FromMajor(50), RoundID: "zz-insufficient"})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("期望 ErrInsufficientBalance，实际: %v", err)
	}

	var cnt int64
	if err := p.db.QueryRow("SELECT COUNT(*) FROM game_transactions WHERE merchant_id = ? AND round_id = ?",
		p.mc, "zz-insufficient").Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("被拒绝的账变不应留下流水，实际 %d 条", cnt)
	}
}

// TestLedgerWalletIsAuthoritativeWhenMirrorIsEmpty —— 玩家首次出现时镜像为 0，
// 但钱包已经扣款成功：此时绝不能因为"本地余额不足"而拒绝记账，
// 否则这笔真实资金变动在账本上完全消失。应当以钱包为准 + 记差额。
func TestLedgerWalletIsAuthoritativeWhenMirrorIsEmpty(t *testing.T) {
	p := newLedgerProbe(t)

	// 镜像初始为 0（全新玩家），钱包说扣款后余额是 900
	wallet := money.FromMajor(900)
	res, err := p.post(Posting{
		TypeCode:      model.TxTypeBet,
		Amount:        money.FromMajor(100),
		RoundID:       "zz-first-bet",
		WalletBalance: &wallet,
	})
	if err != nil {
		t.Fatalf("钱包已确认扣款时不应拒绝记账: %v", err)
	}
	if res.Entry.BalanceAfter != wallet.Minor() {
		t.Errorf("balance_after 应以钱包为准 = %d，实际 %d", wallet.Minor(), res.Entry.BalanceAfter)
	}
	if res.Drift.Minor() == 0 {
		t.Error("镜像为 0 而钱包为 900，应当记出不可解释差额（说明镜像不完整）")
	}
	if got := p.balance(); got != wallet.Minor() {
		t.Errorf("账户镜像应被钱包值修正为 %d，实际 %d", wallet.Minor(), got)
	}
}

// TestLedgerFailsClosedOnUnknownType —— 未知/停用类型必须熔断，
// 绝不"猜一个方向"继续记账（记反账比拒绝一次请求严重得多）。
func TestLedgerFailsClosedOnUnknownType(t *testing.T) {
	p := newLedgerProbe(t)

	if _, err := p.post(Posting{TypeCode: "NO_SUCH_TYPE", Amount: money.FromMajor(1)}); !errors.Is(err, model.ErrTransactionTypeNotFound) {
		t.Fatalf("未知类型期望 ErrTransactionTypeNotFound，实际: %v", err)
	}

	// 停用后再投必须也被拒（缓存 30s，这里显式失效）
	code := "ZZ_LEDGER_DISABLED"
	if _, err := p.db.Exec(
		`INSERT INTO transaction_types (code, scope, io_type, balance_change, frozen_change, name, status)
		 VALUES (?, 'player', 'IN', 'INCREASE', 'NONE', '探针停用类型', 1)
		 ON DUPLICATE KEY UPDATE status = 1`, code); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.db.Exec("DELETE FROM transaction_types WHERE code = ?", code) })

	if _, err := p.post(Posting{TypeCode: code, Amount: money.FromMajor(1)}); err != nil {
		t.Fatalf("启用状态应当可用: %v", err)
	}
	if _, err := p.db.Exec("UPDATE transaction_types SET status = 0 WHERE code = ?", code); err != nil {
		t.Fatal(err)
	}
	p.poster.InvalidateTypes()
	if _, err := p.post(Posting{TypeCode: code, Amount: money.FromMajor(1)}); !errors.Is(err, model.ErrTransactionTypeNotFound) {
		t.Fatalf("停用类型期望被熔断，实际: %v", err)
	}
}

// TestLedgerRejectsNonPositiveAmount —— 金额必须为正：方向由类型表决定，
// 传负数会让"方向"和"符号"两处同时表达，必然出错。
func TestLedgerRejectsNonPositiveAmount(t *testing.T) {
	p := newLedgerProbe(t)
	for _, amt := range []money.Amount{0, -100} {
		if _, err := p.post(Posting{TypeCode: model.TxTypeAdjustAdd, Amount: amt}); !errors.Is(err, ErrAmountNotPositive) {
			t.Fatalf("金额 %d 期望 ErrAmountNotPositive，实际: %v", amt, err)
		}
	}
}

// TestLedgerDetectsUnexplainedDrift —— 影子账的核心能力：
// 钱包返回的余额若与本地推算不一致，差额必须被记录下来。
func TestLedgerDetectsUnexplainedDrift(t *testing.T) {
	p := newLedgerProbe(t)
	base := money.FromMajor(100)
	if _, err := p.post(Posting{TypeCode: model.TxTypeAdjustAdd, Amount: base,
		WalletBalance: ptr(base)}); err != nil {
		t.Fatal(err)
	}

	// 本地推算下注 30 后应为 70，但钱包说余额是 65 —— 少 5，属于不可解释差额
	bet := money.FromMajor(30)
	wallet := money.FromMajor(65)
	res, err := p.post(Posting{TypeCode: model.TxTypeBet, Amount: bet, RoundID: "zz-drift-1",
		WalletBalance: &wallet})
	if err != nil {
		t.Fatalf("BET 失败: %v", err)
	}
	if res.Drift.Minor() != -money.FromMajor(5).Minor() {
		t.Fatalf("差额 = %d，期望 %d", res.Drift.Minor(), -money.FromMajor(5).Minor())
	}
	if res.Entry.BalanceAfter != wallet.Minor() {
		t.Errorf("有差额时 balance_after 应以钱包为准 = %d，实际 %d", wallet.Minor(), res.Entry.BalanceAfter)
	}
	if got := p.balance(); got != wallet.Minor() {
		t.Errorf("账户镜像应以钱包为准 = %d，实际 %d", wallet.Minor(), got)
	}

	var driftCount uint64
	var lastDrift int64
	if err := p.db.QueryRow("SELECT drift_count, last_drift_minor FROM player_accounts WHERE merchant_id = ? AND user_id = ?",
		p.mc, p.user).Scan(&driftCount, &lastDrift); err != nil {
		t.Fatal(err)
	}
	if driftCount != 1 || lastDrift != res.Drift.Minor() {
		t.Errorf("账户差额统计 = (count=%d, last=%d)，期望 (1, %d)", driftCount, lastDrift, res.Drift.Minor())
	}
}

// TestLedgerPendingRetryDoesNotTouchBalance —— 钱包失败/超时的账变只留痕，
// 余额不动（钱到底动没动由补偿链路确认）。
func TestLedgerPendingRetryDoesNotTouchBalance(t *testing.T) {
	p := newLedgerProbe(t)
	base := money.FromMajor(40)
	if _, err := p.post(Posting{TypeCode: model.TxTypeAdjustAdd, Amount: base,
		WalletBalance: ptr(base)}); err != nil {
		t.Fatal(err)
	}

	res, err := p.post(Posting{
		TypeCode: model.TxTypeWin,
		Amount:   money.FromMajor(500),
		RoundID:  "zz-pending-1",
		Status:   model.LedgerStatusPendingRetry,
		Remark:   "钱包超时，待补偿",
	})
	if err != nil {
		t.Fatalf("PENDING_RETRY 入账失败: %v", err)
	}
	if res.Entry.Status != model.LedgerStatusPendingRetry {
		t.Errorf("状态 = %s", res.Entry.Status)
	}
	if got := p.balance(); got != base.Minor() {
		t.Fatalf("待补偿账变不应改动余额：%d，期望 %d", got, base.Minor())
	}
	if res.Entry.BalanceBefore != res.Entry.BalanceAfter {
		t.Errorf("未动钱时前后快照应相同: %d -> %d", res.Entry.BalanceBefore, res.Entry.BalanceAfter)
	}
}

// TestLedgerSnapshotsAreContinuous —— 连续账变的前后快照必须首尾相接。
// 这正是影子账"任意时间点可核对"的基础。
func TestLedgerSnapshotsAreContinuous(t *testing.T) {
	p := newLedgerProbe(t)

	ops := []struct {
		code string
		amt  money.Amount
	}{
		{model.TxTypeAdjustAdd, money.FromMajor(500)},
		{model.TxTypeBet, money.FromMajor(100)},
		{model.TxTypeWin, money.FromMajor(250)},
		{model.TxTypeAdjustSub, money.FromMajor(50)},
	}
	round := "zz-cont-1"
	for i, op := range ops {
		if _, err := p.post(Posting{
			TypeCode: op.code,
			Amount:   op.amt,
			RoundID:  fmt.Sprintf("%s-%d", round, i),
		}); err != nil {
			t.Fatalf("第 %d 笔 (%s) 失败: %v", i, op.code, err)
		}
	}

	rows, err := p.db.Query(
		`SELECT balance_before, balance_after, direction, amount FROM game_transactions
		 WHERE merchant_id = ? AND user_id = ? ORDER BY id ASC`, p.mc, p.user)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var prevAfter int64
	var expected int64
	idx := 0
	for rows.Next() {
		var before, after, amount int64
		var direction string
		if err := rows.Scan(&before, &after, &direction, &amount); err != nil {
			t.Fatal(err)
		}
		if idx > 0 && before != prevAfter {
			t.Errorf("第 %d 笔 before=%d 与上一笔 after=%d 不连续", idx, before, prevAfter)
		}
		if direction == model.LedgerDirectionIn {
			expected = before + amount
		} else {
			expected = before - amount
		}
		if after != expected {
			t.Errorf("第 %d 笔 after=%d，按方向推算应为 %d", idx, after, expected)
		}
		prevAfter = after
		idx++
	}
	if idx != len(ops) {
		t.Fatalf("流水条数 = %d，期望 %d", idx, len(ops))
	}
}

// TestLedgerPostInTxRollsBackWithBusiness —— 账变必须和业务写同事务：
// 业务失败时账变一起回滚，不能出现"钱记了但业务没记"。
func TestLedgerPostInTxRollsBackWithBusiness(t *testing.T) {
	p := newLedgerProbe(t)
	ctx := context.Background()

	sentinel := errors.New("业务失败，整体回滚")
	err := p.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		conn := sqlx.NewSqlConnFromSession(session)
		if _, err := p.poster.PostInTx(ctx, conn, session, Posting{
			TypeCode:     model.TxTypeAdjustAdd,
			MerchantID:   p.mc,
			MerchantCode: "zz-ledger",
			UserID:       p.user,
			Amount:       money.FromMajor(10),
			RoundID:      "zz-tx-rollback",
		}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("期望业务错误，实际: %v", err)
	}

	var cnt int64
	if err := p.db.QueryRow("SELECT COUNT(*) FROM game_transactions WHERE merchant_id = ?", p.mc).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("事务回滚后仍有 %d 条账变流水", cnt)
	}
	if err := p.db.QueryRow("SELECT COUNT(*) FROM player_accounts WHERE merchant_id = ?", p.mc).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("事务回滚后仍留下 %d 个账户", cnt)
	}
}

func ptr(a money.Amount) *money.Amount { return &a }
