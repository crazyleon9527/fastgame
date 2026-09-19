package model

// ledger_schema_test.go —— 账变体系的结构与种子数据防线
//
// 账变两张表 + 类型表有三类问题编译期看不出来，只能靠真库校验：
//  1. 模型字段与表列漂移（加列后忘了改模型 → 运行期 Scan 报错）；
//  2. 类型表的种子数据被改坏（例如把 BET 的方向改成 INCREASE，
//     那就是"下注变加钱"，而且只会在真实下注时才发现）；
//  3. 幂等键丢了终态列（补偿流水会被当成重复丢掉 → 钱到了账上没有）。
//
// 未设置 MYSQL_DSN 且本地 MySQL 不可达时自动跳过。

import (
	"database/sql"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/builder"
)

// TestLedgerModelStructsMatchLiveTables —— 手写模型的字段必须是真实列的子集。
func TestLedgerModelStructsMatchLiveTables(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	cases := []struct {
		table  string
		fields []string
	}{
		{"transaction_types", builder.RawFieldNames(&TransactionType{})},
		{"game_transactions", builder.RawFieldNames(&GameTransaction{})},
		{"player_accounts", builder.RawFieldNames(&PlayerAccount{})},
	}
	for _, c := range cases {
		live := liveColumns(t, db, c.table)
		for _, f := range c.fields {
			name := normalizeCol(f)
			if !live[name] {
				t.Errorf("表 %s：模型字段 %s 在实际表中不存在（迁移 35 之后的模型漂移）", c.table, name)
			}
		}
	}
}

// TestLedgerTypeSeedDirections —— 种子类型的方向必须是"账本正确性"的那个方向。
//
// 这些方向就是钱的方向：改错一个，账本记的就是反账，而且不会有任何报错。
func TestLedgerTypeSeedDirections(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	want := map[string]struct {
		ioType        string
		balanceChange string
	}{
		TxTypeBet:         {"OUT", ChangeDecrease},
		TxTypeWin:         {"IN", ChangeIncrease},
		TxTypeRefund:      {"IN", ChangeIncrease},
		TxTypeRollback:    {"IN", ChangeIncrease},
		TxTypePromoCredit: {"IN", ChangeIncrease},
		TxTypeAdjustAdd:   {"IN", ChangeIncrease},
		TxTypeAdjustSub:   {"OUT", ChangeDecrease},
	}

	for code, exp := range want {
		var ioType, balanceChange, frozenChange string
		var status int64
		err := db.QueryRow(
			`SELECT io_type, balance_change, frozen_change, status FROM transaction_types WHERE code = ?`, code).
			Scan(&ioType, &balanceChange, &frozenChange, &status)
		if err == sql.ErrNoRows {
			t.Errorf("账变类型 %s 未种子化（迁移 35 的 INSERT 被漏掉或被删）", code)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if ioType != exp.ioType || balanceChange != exp.balanceChange {
			t.Errorf("类型 %s 方向 = (%s, %s)，期望 (%s, %s) —— 方向错了就是记反账",
				code, ioType, balanceChange, exp.ioType, exp.balanceChange)
		}
		if frozenChange != ChangeNone {
			t.Errorf("类型 %s 的 frozen_change = %s，玩家维度期望 NONE", code, frozenChange)
		}
		if status != 1 {
			t.Errorf("类型 %s 的 status = %d，期望 1（停用会导致账变被熔断）", code, status)
		}
	}
}

// TestLedgerIdempotencyKeyIncludesStatus —— 幂等键必须包含 status。
//
// 少了 status，补偿流水（同一局的 WIN 从 PENDING_RETRY 变为 SUCCESS）
// 会被唯一键判成重复而丢弃，结果是"钱到了但账上没有"。
func TestLedgerIdempotencyKeyIncludesStatus(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	rows, err := db.Query(
		`SELECT COLUMN_NAME FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'game_transactions'
		   AND INDEX_NAME = 'uk_merchant_round_type_status'
		 ORDER BY SEQ_IN_INDEX`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, normalizeCol(c))
	}
	if len(cols) == 0 {
		t.Fatal("找不到幂等键 uk_merchant_round_type_status（迁移 36 未应用？）")
	}
	want := []string{"merchant_id", "round_id", "tx_type", "status"}
	if len(cols) != len(want) {
		t.Fatalf("幂等键列 = %v，期望 %v", cols, want)
	}
	for i := range want {
		if cols[i] != want[i] {
			t.Fatalf("幂等键列 = %v，期望 %v", cols, want)
		}
	}
}

// normalizeCol 去掉 builder 返回的反引号并统一小写。
func normalizeCol(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '`' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}
