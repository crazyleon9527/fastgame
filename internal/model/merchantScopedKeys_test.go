package model

// merchant_scoped_keys_test.go — 迁移 33 的回归测试
//
// 迁移 33 把三张对账表的唯一键改成 merchant_id 前导：
//
//	pending_transactions  uk_merchant_round(merchant_id, round_id)
//	game_round_replay     uk_merchant_round(merchant_id, round_id)
//	wallet_pending_ops    uk_merchant_round_op(merchant_id, round_id, op_type)
//
// 这带来两个必须被测试守住的行为：
//
//  1. 不同商户可以合法使用同一个 round_id（唯一键不再跨商户撞车）——这是迁移的目的；
//  2. 凡是按 round_id 定位行的读写都必须带上 merchant_id，否则会命中别的商户。
//     同时 uk_round_id 已被删除，只按 round_id 查会用不上索引。
//
// 运行（需要 fastgame-mysql 容器在线）：
//   go test ./internal/model/ -run TestMerchantScoped -v
// MySQL 不可达时自动跳过。

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 用远离种子数据的商户 id，避免与 merchants 表的外键/业务数据纠缠
const (
	probeMerchantA uint64 = 900001
	probeMerchantB uint64 = 900002
)

func probeConn(t *testing.T) sqlx.SqlConn {
	t.Helper()
	db := openTestDB(t) // 不可达时 t.Skip
	db.Close()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	return sqlx.NewMysql(dsn)
}

func cleanupPendingRounds(t *testing.T, db *sql.DB, pattern string) {
	t.Helper()
	t.Cleanup(func() {
		db.Exec("DELETE FROM pending_transactions WHERE round_id LIKE ?", pattern)
		db.Exec("DELETE FROM game_round_replay WHERE round_id LIKE ?", pattern)
		db.Exec("DELETE FROM wallet_pending_ops WHERE round_id LIKE ?", pattern)
	})
	db.Exec("DELETE FROM pending_transactions WHERE round_id LIKE ?", pattern)
	db.Exec("DELETE FROM game_round_replay WHERE round_id LIKE ?", pattern)
	db.Exec("DELETE FROM wallet_pending_ops WHERE round_id LIKE ?", pattern)
}

// TestMerchantScopedPendingTransactions 覆盖 pending_transactions：
// 同一 round_id 在两个商户下都能落库，且 MarkSettled 只影响指定商户。
func TestMerchantScopedPendingTransactions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	conn := probeConn(t)
	ctx := context.Background()

	const roundID = "zz-probe-merchant-scope-1"
	cleanupPendingRounds(t, db, "zz-probe-merchant-scope%")

	m := NewPendingTransactionsModel(conn)
	insert := func(merchantID uint64, code string) {
		t.Helper()
		if err := m.Insert(ctx, &PendingTransaction{
			TraceID:         "zz-probe-trace",
			RoundID:         roundID,
			MerchantID:      merchantID,
			MerchantCode:    code,
			UserID:          "zz-probe-user",
			GameCode:        "fishing",
			Phase:           PendingTxPhaseBetDebited,
			Status:          PendingTxStatusPending,
			BetAmount:       10000,
			ExpectedAction:  PendingTxActionRollbackBet,
			WalletBetStatus: "confirmed",
			WalletWinStatus: "unknown",
		}); err != nil {
			t.Fatalf("Insert(merchant=%d) 失败: %v", merchantID, err)
		}
	}

	// 1) 同一 round_id、两个商户 —— 迁移 33 之后必须都能写入
	insert(probeMerchantA, "zz-probe-a")
	insert(probeMerchantB, "zz-probe-b")

	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pending_transactions WHERE round_id = ?", roundID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("同一 round_id 在两个商户下应落 2 行，实际 %d 行 —— "+
			"唯一键可能仍是 round_id 单列（见迁移 33）", n)
	}

	// 2) merchant_id 必须真的写进去，而不是落成列默认值 0
	var gotA uint64
	if err := db.QueryRowContext(ctx,
		"SELECT merchant_id FROM pending_transactions WHERE round_id = ? AND merchant_code = 'zz-probe-a'",
		roundID).Scan(&gotA); err != nil {
		t.Fatal(err)
	}
	if gotA != probeMerchantA {
		t.Fatalf("merchant_id = %d，期望 %d —— Insert 漏写 merchant_id 会让唯一键退化成 (0, round_id)",
			gotA, probeMerchantA)
	}

	// 3) MarkSettled 必须只改指定商户那一行
	if err := m.MarkSettled(ctx, probeMerchantA, roundID); err != nil {
		t.Fatalf("MarkSettled 失败: %v", err)
	}
	phaseOf := func(code string) string {
		t.Helper()
		var phase string
		if err := db.QueryRowContext(ctx,
			"SELECT phase FROM pending_transactions WHERE round_id = ? AND merchant_code = ?",
			roundID, code).Scan(&phase); err != nil {
			t.Fatal(err)
		}
		return phase
	}
	if got := phaseOf("zz-probe-a"); got != PendingTxPhaseSettled {
		t.Errorf("商户 A 的 phase = %q，期望 %q", got, PendingTxPhaseSettled)
	}
	if got := phaseOf("zz-probe-b"); got != PendingTxPhaseBetDebited {
		t.Errorf("商户 B 的 phase 被改成了 %q —— MarkSettled 按 round_id 单列更新会串商户数据", got)
	}

	// 4) FindByRoundID 按商户定位；merchantID=0 表示不限商户
	rowA, err := m.FindByRoundID(ctx, probeMerchantA, roundID)
	if err != nil {
		t.Fatalf("FindByRoundID(A) 失败: %v", err)
	}
	if rowA.MerchantID != probeMerchantA {
		t.Errorf("FindByRoundID(A) 返回 merchant_id=%d，期望 %d", rowA.MerchantID, probeMerchantA)
	}
	if _, err := m.FindByRoundID(ctx, 0, roundID); err != nil {
		t.Errorf("FindByRoundID(0)（不限商户）失败: %v", err)
	}
}

// TestMerchantScopedGameRoundReplay 覆盖 game_round_replay：
// 同一 round_id 在两个商户下都能落库，FindByRoundID 按商户取。
func TestMerchantScopedGameRoundReplay(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	conn := probeConn(t)
	ctx := context.Background()

	const roundID = "zz-probe-merchant-scope-2"
	cleanupPendingRounds(t, db, "zz-probe-merchant-scope%")

	m := NewGameRoundReplayModel(conn)
	for _, tc := range []struct {
		id   uint64
		code string
		seed string
	}{
		{probeMerchantA, "zz-probe-a", "server-seed-a"},
		{probeMerchantB, "zz-probe-b", "server-seed-b"},
	} {
		if err := m.Insert(ctx, &GameRoundReplay{
			RoundID:      roundID,
			MerchantID:   tc.id,
			MerchantCode: tc.code,
			UserID:       "zz-probe-user",
			GameCode:     "fishing",
			ServerSeed:   tc.seed,
			ClientSeed:   "client",
			Nonce:        roundID,
			BetAmount:    10000,
			SequenceID:   1,
		}); err != nil {
			t.Fatalf("Insert(merchant=%d) 失败: %v", tc.id, err)
		}
	}

	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM game_round_replay WHERE round_id = ?", roundID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("同一 round_id 在两个商户下应落 2 行，实际 %d 行", n)
	}

	rowA, err := m.FindByRoundID(ctx, probeMerchantA, roundID)
	if err != nil {
		t.Fatalf("FindByRoundID(A) 失败: %v", err)
	}
	if rowA.MerchantID != probeMerchantA || rowA.ServerSeed != "server-seed-a" {
		t.Errorf("FindByRoundID(A) 取到了 merchant_id=%d seed=%q —— 应为商户 A 的记录",
			rowA.MerchantID, rowA.ServerSeed)
	}

	rowB, err := m.FindByRoundID(ctx, probeMerchantB, roundID)
	if err != nil {
		t.Fatalf("FindByRoundID(B) 失败: %v", err)
	}
	if rowB.ServerSeed != "server-seed-b" {
		t.Errorf("FindByRoundID(B) 取到了 seed=%q —— 应为商户 B 的记录", rowB.ServerSeed)
	}
}

// TestMerchantScopedWalletPendingOps 覆盖 wallet_pending_ops：
// merchant_id 要落库，FindByRoundOp 按 (merchant_id, round_id, op_type) 定位。
func TestMerchantScopedWalletPendingOps(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	conn := probeConn(t)
	ctx := context.Background()

	const roundID = "zz-probe-merchant-scope-3"
	cleanupPendingRounds(t, db, "zz-probe-merchant-scope%")

	m := NewWalletPendingOpsModel(conn)
	for _, tc := range []struct {
		id   uint64
		code string
	}{
		{probeMerchantA, "zz-probe-a"},
		{probeMerchantB, "zz-probe-b"},
	} {
		if _, err := m.Insert(ctx, &WalletPendingOp{
			RoundID:      roundID,
			MerchantID:   tc.id,
			MerchantCode: tc.code,
			UserID:       "zz-probe-user",
			OpType:       PendingOpWinFailed,
			BetAmount:    10000,
			WinAmount:    50000,
			Status:       PendingStatusPending,
		}); err != nil {
			t.Fatalf("Insert(merchant=%d) 失败: %v", tc.id, err)
		}
	}

	opA, err := m.FindByRoundOp(ctx, probeMerchantA, roundID, PendingOpWinFailed)
	if err != nil {
		t.Fatalf("FindByRoundOp(A) 失败: %v", err)
	}
	if opA.MerchantID != probeMerchantA {
		t.Errorf("FindByRoundOp(A) 返回 merchant_id=%d，期望 %d", opA.MerchantID, probeMerchantA)
	}

	opB, err := m.FindByRoundOp(ctx, probeMerchantB, roundID, PendingOpWinFailed)
	if err != nil {
		t.Fatalf("FindByRoundOp(B) 失败: %v", err)
	}
	if opB.MerchantID != probeMerchantB {
		t.Errorf("FindByRoundOp(B) 返回 merchant_id=%d，期望 %d", opB.MerchantID, probeMerchantB)
	}
}
