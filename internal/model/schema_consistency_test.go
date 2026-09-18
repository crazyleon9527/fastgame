package model

// schema_consistency_test.go — 模型与实际表结构的一致性校验
//
// 这个测试解决的问题：goctl 生成的 *_gen.go 用 builder.RawFieldNames(&Struct{})
// 动态拼 SELECT/INSERT/UPDATE 的列名，一旦迁移加了列而模型没重新生成，
// 生成的 SQL 就会缺列，且**编译期完全看不出来**。
//
// 运行（需要 fastgame-mysql 容器在线）：
//   go test ./internal/model/ -run TestSchema -v
// 未设置 MYSQL_DSN 时自动跳过，因此不会影响常规 `go test ./...`。

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/builder"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const defaultDSN = "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("跳过：无法创建 MySQL 连接（%v）", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("跳过：MySQL 不可达（%v）。先执行 docker compose up -d mysql", err)
	}
	return db
}

// liveColumns 读取实际表的列集合（小写）
func liveColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT COLUMN_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table)
	if err != nil {
		t.Fatalf("查询 %s 列失败: %v", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		out[strings.ToLower(c)] = true
	}
	if len(out) == 0 {
		t.Fatalf("表 %s 不存在或没有列", table)
	}
	return out
}

// checkGeneratedModel 校验 goctl 生成模型声明的列与实际表列**完全一致**
func checkGeneratedModel(t *testing.T, db *sql.DB, table string, fields []string) {
	t.Helper()
	live := liveColumns(t, db, table)
	model := map[string]bool{}
	for _, f := range fields {
		model[strings.ToLower(strings.Trim(f, "`"))] = true
	}

	var missing, extra []string
	for c := range live {
		if !model[c] {
			missing = append(missing, c)
		}
	}
	for c := range model {
		if !live[c] {
			extra = append(extra, c)
		}
	}
	if len(missing) > 0 {
		t.Errorf("表 %s：生成模型缺少 %d 列 %v —— 表结构变更后需要重新运行 goctl 或手工补字段", table, len(missing), missing)
	}
	if len(extra) > 0 {
		t.Errorf("表 %s：生成模型多出 %d 列 %v —— 这些列在数据库中不存在，生成的 SQL 会报错", table, len(extra), extra)
	}
}

func TestSchemaGeneratedModelsMatchLiveTables(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	checkGeneratedModel(t, db, "merchants", builder.RawFieldNames(&Merchants{}))
	checkGeneratedModel(t, db, "admin_users", builder.RawFieldNames(&AdminUsers{}))
	checkGeneratedModel(t, db, "game_configs", builder.RawFieldNames(&GameConfigs{}))
}

// TestSchemaCustomModelStructsSubsetOfLive
// 手写模型的结构体字段必须是实际列的子集（多出来的列会在 Scan 时报错）
func TestSchemaCustomModelStructsSubsetOfLive(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	cases := []struct {
		table  string
		fields []string
	}{
		{"admin_users", builder.RawFieldNames(&AdminAuthRecord{})},
		{"audit_logs", builder.RawFieldNames(&AuditLog{})},
		{"daily_settlements", builder.RawFieldNames(&DailySettlement{})},
		{"game_round_replay", builder.RawFieldNames(&GameRoundReplay{})},
		{"pending_transactions", builder.RawFieldNames(&PendingTransaction{})},
		{"risk_alerts", builder.RawFieldNames(&RiskAlert{})},
		{"risk_blacklist", builder.RawFieldNames(&RiskBlacklist{})},
		{"roles", builder.RawFieldNames(&RoleRow{})},
		{"settlement_periods", builder.RawFieldNames(&SettlementPeriod{})},
		{"wallet_pending_ops", builder.RawFieldNames(&WalletPendingOp{})},
	}
	for _, c := range cases {
		live := liveColumns(t, db, c.table)
		for _, f := range c.fields {
			name := strings.ToLower(strings.Trim(f, "`"))
			if !live[name] {
				t.Errorf("表 %s：模型字段 %s 在实际表中不存在", c.table, name)
			}
		}
	}
}

// liveColumnTypes 读取实际列的 data_type（小写）
func liveColumnTypes(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT COLUMN_NAME, DATA_TYPE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table)
	if err != nil {
		t.Fatalf("查询 %s 列类型失败: %v", table, err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var c, ty string
		if err := rows.Scan(&c, &ty); err != nil {
			t.Fatal(err)
		}
		out[strings.ToLower(c)] = strings.ToLower(ty)
	}
	return out
}

// TestSchemaStringIDColumnsAreVarchar
//
// 类型级校验：Go 模型里声明为 string 的 ID 列，数据库必须是字符型。
// 只比对列名不够——把 Go 的 uint64 改成 string 而库里仍是 BIGINT 时，
// 列名比对会通过，但运行期 Scan 会失败（这就是 platform-api 里
// user.status 声明 varchar(191) 而 Go 用 uint8 的同类漂移）。
func TestSchemaStringIDColumnsAreVarchar(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Go 侧为 string 的 ID 列 —— 与 migration 10-user-id-string（USER_ID 统一为 VARCHAR(64)）对应
	requirements := []struct {
		table  string
		column string
	}{
		{"pending_transactions", "user_id"},
		{"wallet_pending_ops", "user_id"},
		{"game_round_replay", "user_id"},
	}

	charTypes := map[string]bool{"varchar": true, "char": true, "text": true, "longtext": true, "mediumtext": true}
	for _, r := range requirements {
		types := liveColumnTypes(t, db, r.table)
		got, ok := types[r.column]
		if !ok {
			t.Errorf("表 %s 缺少列 %s", r.table, r.column)
			continue
		}
		if !charTypes[got] {
			t.Errorf("表 %s.%s 数据库类型是 %s，但 Go 模型声明为 string —— "+
				"运行期 Scan 会失败。请应用 docker/mysql/init/10-user-id-string-migration.sql",
				r.table, r.column, got)
		}
	}
}

// TestSchemaGeneratedSQLExecutes 真跑一遍生成的 SQL，确保列名与占位符数量都正确。
// goctl 生成代码里 Insert/Update 的 "?, ?, ?" 是硬编码的，加字段时极易漏改。
func TestSchemaGeneratedSQLExecutes(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	conn := sqlx.NewMysql(os.Getenv("MYSQL_DSN"))
	if os.Getenv("MYSQL_DSN") == "" {
		conn = sqlx.NewMysql(defaultDSN)
	}
	ctx := context.Background()

	code := "zz_schema_probe"
	if _, err := db.ExecContext(ctx, "DELETE FROM merchants WHERE merchant_code = ?", code); err != nil {
		t.Fatalf("清理失败: %v", err)
	}

	m := NewMerchantsModel(conn)
	id, err := m.Insert(ctx, &Merchants{
		MerchantCode: code,
		Name:         "schema probe",
		Status:       1,
	})
	if err != nil {
		t.Fatalf("Merchants.Insert 执行失败（生成的 SQL 与表结构不一致）: %v", err)
	}
	newID, _ := id.LastInsertId()
	t.Cleanup(func() { db.Exec("DELETE FROM merchants WHERE id = ?", newID) })

	row, err := m.FindOne(ctx, uint64(newID))
	if err != nil {
		t.Fatalf("Merchants.FindOne 失败: %v", err)
	}
	if row.MerchantCode != code {
		t.Errorf("FindOne 返回 merchant_code=%q，期望 %q", row.MerchantCode, code)
	}
	if row.AllowedIPs.Valid == false && row.AllowedIPs.String != "" {
		t.Log("allowed_ips 为空，符合预期")
	}

	if err := m.Update(ctx, row); err != nil {
		t.Fatalf("Merchants.Update 执行失败（生成的占位符数量可能不正确）: %v", err)
	}

	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM merchants WHERE merchant_code = ?", code).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("期望 1 行，实际 %d", n)
	}
	t.Logf("merchants 生成的 CRUD SQL 全部执行成功（id=%d）", newID)
}

// TestSchemaAdminUsersGeneratedSQLExecutes
// admin_users 的生成模型曾遗漏 totp_* 三列，导致 Insert 的列数与占位符数不匹配。
func TestSchemaAdminUsersGeneratedSQLExecutes(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	conn := sqlx.NewMysql(dsn)
	ctx := context.Background()

	name := "zz_schema_probe"
	db.ExecContext(ctx, "DELETE FROM admin_users WHERE username = ?", name)

	var roleID uint64
	if err := db.QueryRowContext(ctx, "SELECT id FROM roles ORDER BY id LIMIT 1").Scan(&roleID); err != nil {
		t.Skipf("跳过：roles 表为空（%v）", err)
	}

	m := NewAdminUsersModel(conn)
	res, err := m.Insert(ctx, &AdminUsers{
		Username:     name,
		PasswordHash: "x",
		RoleId:       roleID,
		Status:       1,
	})
	if err != nil {
		t.Fatalf("AdminUsers.Insert 执行失败（生成的 SQL 与表结构不一致）: %v", err)
	}
	newID, _ := res.LastInsertId()
	t.Cleanup(func() { db.Exec("DELETE FROM admin_users WHERE id = ?", newID) })

	row, err := m.FindOne(ctx, uint64(newID))
	if err != nil {
		t.Fatalf("AdminUsers.FindOne 失败: %v", err)
	}
	if row.Username != name {
		t.Errorf("FindOne 返回 username=%q，期望 %q", row.Username, name)
	}
	if err := m.Update(ctx, row); err != nil {
		t.Fatalf("AdminUsers.Update 执行失败: %v", err)
	}
	t.Logf("admin_users 生成的 CRUD SQL 全部执行成功（id=%d, totp_enabled=%d）", newID, row.TotpEnabled)
}

// TestSchemaGameConfigsGeneratedSQLExecutes
func TestSchemaGameConfigsGeneratedSQLExecutes(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	conn := sqlx.NewMysql(dsn)
	ctx := context.Background()

	const key = "zz_schema_probe"
	db.ExecContext(ctx, "DELETE FROM game_configs WHERE config_key = ?", key)
	t.Cleanup(func() { db.Exec("DELETE FROM game_configs WHERE config_key = ?", key) })

	m := NewGameConfigsModel(conn)
	res, err := m.Insert(ctx, &GameConfigs{
		MerchantId:  1,
		GameCode:    "zz_probe",
		ConfigKey:   key,
		ConfigValue: `{"probe":true}`,
		Status:      1,
	})
	if err != nil {
		t.Fatalf("GameConfigs.Insert 执行失败: %v", err)
	}
	newID, _ := res.LastInsertId()
	row, err := m.FindOne(ctx, uint64(newID))
	if err != nil {
		t.Fatalf("GameConfigs.FindOne 失败: %v", err)
	}
	if row.ConfigKey != key {
		t.Errorf("FindOne 返回 config_key=%q，期望 %q", row.ConfigKey, key)
	}
	if err := m.Update(ctx, row); err != nil {
		t.Fatalf("GameConfigs.Update 执行失败: %v", err)
	}
	t.Logf("game_configs 生成的 CRUD SQL 全部执行成功（id=%d）", newID)
}
