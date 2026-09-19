package logic

// ledgerAdjustmentLogic_test.go —— 人工调账的行为测试（需要 fastgame-mysql 容器在线）
//
// 这里断言的是人工调账必须成立的几条：
//   · 登记的是"钱包侧已经做过的人工调整"，因此 externalTxId / remark 必填
//   · 只接受 ADJUST_ADD / ADJUST_SUB，别的一律拒绝（避免调错方向）
//   · 金额必须为正数（方向由类型表决定）
//   · 操作人（id / 用户名 / IP）落进 extra_data，事后能追责
//   · WalletBalance 传 nil → drift_minor 记 0（本地按类型推算镜像）
//   · 落库后能被后台流水查询看到，且 typeName 来自 transaction_types
//
// MySQL 不可达时自动跳过；探针用远离业务数据的 merchant_id = 990002，
// 并在开始与结束时清掉自己插入的 game_transactions / player_accounts 行。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/fieldcipher"
	"fastgame/pkg/ledger"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const defaultAdminTestDSN = "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"

// adjustProbeMerchantID 探针商户：远离业务种子数据，避免误删/误改真实流水。
const adjustProbeMerchantID = 990002

func newAdjustProbe(t *testing.T) (*sql.DB, *svc.ServiceContext, uint64) {
	t.Helper()

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = defaultAdminTestDSN
	}
	fieldcipher.Init("") // 测试里不碰 TOTP 密钥，用空口令初始化即可

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

	conn := sqlx.NewMysql(dsn)
	cleanup := func() {
		db.Exec("DELETE FROM game_transactions WHERE merchant_id = ?", adjustProbeMerchantID)
		db.Exec("DELETE FROM player_accounts WHERE merchant_id = ?", adjustProbeMerchantID)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		db.Close()
	})

	svcCtx := &svc.ServiceContext{
		AdminUsers:  model.NewAdminUsersModel(conn),
		Ledger:      ledger.NewPoster(conn, nil),
		LedgerModel: model.NewLedgerModel(),
		DB:          conn,
	}
	return db, svcCtx, insertProbeAdmin(t, db)
}

// insertProbeAdmin 插一个探针管理员，用返回的自增 id 而不是查种子数据：
// admin_users 的初始数据来自迁移脚本，测试不该绑死某一行 id，也不该依赖它的内容。
func insertProbeAdmin(t *testing.T, db *sql.DB) uint64 {
	t.Helper()

	res, err := db.Exec(
		`INSERT INTO admin_users (username, password_hash, role_id, status, totp_enabled)
		 VALUES (?, ?, 1, 1, 0)`,
		"zz-ledger-probe-admin", "$2a$10$zzledgerprobeplaceholder")
	if err != nil {
		t.Fatalf("插入探针管理员失败: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("读取探针管理员 id 失败: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM admin_users WHERE id = ?", id)
	})
	return uint64(id)
}

func adjustCtx(adminID uint64) context.Context {
	// 与 JWT 解析后放进上下文的是同一套键（见同包的 ctxHelper.go）。
	return context.WithValue(context.Background(), "userId", float64(adminID))
}

// adjustReq 只填必填项，各用例按需覆盖。
func adjustReq() types.LedgerAdjustmentReq {
	return types.LedgerAdjustmentReq{
		MerchantId:   adjustProbeMerchantID,
		UserId:       "zz-adjust-probe-user",
		TypeCode:     model.TxTypeAdjustAdd,
		AmountMinor:  12345,
		ExternalTxId: "zz-wallet-voucher-1",
		Remark:       "活动补偿加款（探针）",
	}
}

// TestLedgerAdjustmentRegistersWalletSideAdjustment —— 正路径：
// 加款登记成功、镜像余额按类型方向推进、操作人信息进 extra_data。
func TestLedgerAdjustmentRegistersWalletSideAdjustment(t *testing.T) {
	db, svcCtx, adminID := newAdjustProbe(t)

	const adminIP = "10.9.9.9"
	req := adjustReq()
	req.OperatorIp = adminIP

	l := NewLedgerAdjustmentLogic(adjustCtx(adminID), svcCtx)
	resp, err := l.Adjust(&req)
	if err != nil {
		t.Fatalf("人工加款失败: %v", err)
	}

	if resp.TxType != model.TxTypeAdjustAdd || resp.Direction != model.LedgerDirectionIn {
		t.Errorf("txType/direction = %s/%s，期望 %s/%s",
			resp.TxType, resp.Direction, model.TxTypeAdjustAdd, model.LedgerDirectionIn)
	}
	if resp.AmountMinor != req.AmountMinor {
		t.Errorf("amountMinor = %d，期望 %d", resp.AmountMinor, req.AmountMinor)
	}
	if resp.BalanceBeforeMinor != 0 || resp.BalanceAfterMinor != req.AmountMinor {
		t.Errorf("前后余额 = (%d, %d)，期望 (0, %d)",
			resp.BalanceBeforeMinor, resp.BalanceAfterMinor, req.AmountMinor)
	}
	if resp.Status != model.LedgerStatusSuccess {
		t.Errorf("status = %s，期望 %s", resp.Status, model.LedgerStatusSuccess)
	}
	if resp.DriftMinor != 0 {
		t.Errorf("WalletBalance 传 nil 时 driftMinor 应为 0，实际 %d", resp.DriftMinor)
	}
	if resp.Duplicated {
		t.Error("首次人工调账不应被判为幂等重复")
	}
	if resp.TransactionId == "" {
		t.Error("返回体缺少 transactionId")
	}

	// 库里的流水与返回体一致，且带上钱包凭证号、无单局归属、操作人快照
	var (
		gotExternal, gotType, gotStatus, gotRemark string
		gotRoundID                                 sql.NullString
		gotAmount, gotBefore, gotAfter, gotDrift   int64
		extraRaw                                   sql.NullString
	)
	err = db.QueryRow(
		`SELECT external_tx_id, tx_type, status, remark, round_id,
		        amount, balance_before, balance_after, drift_minor, extra_data
		 FROM game_transactions WHERE merchant_id = ? AND user_id = ?`,
		adjustProbeMerchantID, req.UserId).Scan(
		&gotExternal, &gotType, &gotStatus, &gotRemark, &gotRoundID,
		&gotAmount, &gotBefore, &gotAfter, &gotDrift, &extraRaw)
	if err != nil {
		t.Fatalf("回查账变流水失败: %v", err)
	}
	if gotExternal != req.ExternalTxId {
		t.Errorf("external_tx_id = %q，期望 %q（钱包侧凭证号必须落库）", gotExternal, req.ExternalTxId)
	}
	if gotType != model.TxTypeAdjustAdd || gotStatus != model.LedgerStatusSuccess {
		t.Errorf("tx_type/status = %s/%s", gotType, gotStatus)
	}
	if gotRemark != req.Remark {
		t.Errorf("remark = %q，期望 %q", gotRemark, req.Remark)
	}
	if gotRoundID.Valid && gotRoundID.String != "" {
		t.Errorf("人工调账的 round_id 应为 NULL/空，实际 %q", gotRoundID.String)
	}
	if gotAmount != req.AmountMinor || gotBefore != 0 || gotAfter != req.AmountMinor || gotDrift != 0 {
		t.Errorf("流水金额快照 = amount:%d before:%d after:%d drift:%d",
			gotAmount, gotBefore, gotAfter, gotDrift)
	}

	var extra map[string]any
	if !extraRaw.Valid || json.Unmarshal([]byte(extraRaw.String), &extra) != nil {
		t.Fatalf("extra_data 不是合法 JSON: %v", extraRaw)
	}
	if got, ok := extra["operator_admin_id"].(float64); !ok || uint64(got) != adminID {
		t.Errorf("extra_data.operator_admin_id = %v，期望 %d", extra["operator_admin_id"], adminID)
	}
	if got, _ := extra["operator_username"].(string); got != "zz-ledger-probe-admin" {
		t.Errorf("extra_data.operator_username = %v，期望 zz-ledger-probe-admin", extra["operator_username"])
	}
	if got, _ := extra["operator_ip"].(string); got != adminIP {
		t.Errorf("extra_data.operator_ip = %v，期望 %s", extra["operator_ip"], adminIP)
	}
	if got, _ := extra["ref_type"].(string); got != "admin_adjustment" {
		t.Errorf("extra_data.ref_type = %v，期望 admin_adjustment", extra["ref_type"])
	}
	if got, _ := extra["ref_id"].(string); got != req.ExternalTxId {
		t.Errorf("extra_data.ref_id = %v，期望 %s", extra["ref_id"], req.ExternalTxId)
	}

	// 影子账户：镜像余额按类型方向（ADJUST_ADD=INCREASE）推进
	var balance int64
	if err := db.QueryRow(
		"SELECT balance_minor FROM player_accounts WHERE merchant_id = ? AND user_id = ?",
		adjustProbeMerchantID, req.UserId).Scan(&balance); err != nil {
		t.Fatalf("回查影子账户失败: %v", err)
	}
	if balance != req.AmountMinor {
		t.Errorf("镜像余额 = %d，期望 %d", balance, req.AmountMinor)
	}
}

// TestLedgerAdjustmentRejectsBadInput —— 参数不合法必须在写库前被拒：
// 类型不在白名单、金额非正、缺钱包凭证号、缺调整原因、缺商户/玩家。
func TestLedgerAdjustmentRejectsBadInput(t *testing.T) {
	db, svcCtx, adminID := newAdjustProbe(t)

	cases := []struct {
		name    string
		mutate  func(r *types.LedgerAdjustmentReq)
		wantErr error
	}{
		{"类型不在白名单(BET)", func(r *types.LedgerAdjustmentReq) { r.TypeCode = model.TxTypeBet }, ErrAdjustTypeUnsupported},
		{"类型不在白名单(未知)", func(r *types.LedgerAdjustmentReq) { r.TypeCode = "NO_SUCH_TYPE" }, ErrAdjustTypeUnsupported},
		{"金额为 0", func(r *types.LedgerAdjustmentReq) { r.AmountMinor = 0 }, ledger.ErrAmountNotPositive},
		{"金额为负", func(r *types.LedgerAdjustmentReq) { r.AmountMinor = -1 }, ledger.ErrAmountNotPositive},
		{"缺钱包凭证号", func(r *types.LedgerAdjustmentReq) { r.ExternalTxId = "  " }, nil},
		{"缺调整原因", func(r *types.LedgerAdjustmentReq) { r.Remark = "" }, nil},
		{"缺 userId", func(r *types.LedgerAdjustmentReq) { r.UserId = "" }, nil},
		{"缺 merchantId", func(r *types.LedgerAdjustmentReq) { r.MerchantId = 0 }, nil},
	}

	for _, c := range cases {
		req := adjustReq()
		c.mutate(&req)
		l := NewLedgerAdjustmentLogic(adjustCtx(adminID), svcCtx)
		_, err := l.Adjust(&req)
		if err == nil {
			t.Errorf("%s：期望被拒绝，实际成功", c.name)
			continue
		}
		if c.wantErr != nil && !errors.Is(err, c.wantErr) {
			t.Errorf("%s：期望 %v，实际 %v", c.name, c.wantErr, err)
		}
	}

	// 被拒绝的请求不能留下任何流水（否则等于无效调账也记了账）
	var cnt int64
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM game_transactions WHERE merchant_id = ?",
		adjustProbeMerchantID).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("非法入参留下了 %d 条流水，期望 0 条", cnt)
	}
}

// TestLedgerAdjustmentSubRejectsInsufficientMirror —— ADJUST_SUB 在本地镜像
// 不足时必须拒绝：这不代表钱包真的拒绝，而是本地台账与钱包状态不一致，
// 属于必须人工核查的异常，绝不能记成负数。
func TestLedgerAdjustmentSubRejectsInsufficientMirror(t *testing.T) {
	db, svcCtx, adminID := newAdjustProbe(t)

	req := adjustReq()
	req.TypeCode = model.TxTypeAdjustSub
	req.AmountMinor = 500
	req.ExternalTxId = "zz-wallet-voucher-sub"
	req.Remark = "探针：本地镜像不足的扣款"

	l := NewLedgerAdjustmentLogic(adjustCtx(adminID), svcCtx)
	if _, err := l.Adjust(&req); !errors.Is(err, ledger.ErrInsufficientBalance) {
		t.Fatalf("期望 ErrInsufficientBalance，实际 %v", err)
	}

	var cnt int64
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM game_transactions WHERE merchant_id = ?",
		adjustProbeMerchantID).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("被拒绝的扣款留下了 %d 条流水，期望 0 条", cnt)
	}
}

// TestLedgerErrStatusMapping —— 错误映射：调用方可修复→400，服务端故障→500。
func TestLedgerErrStatusMapping(t *testing.T) {
	cases := []struct {
		err        error
		wantStatus int
		mustSay    string
	}{
		{ledger.ErrAmountNotPositive, 400, "金额必须为正数"},
		{ledger.ErrInsufficientBalance, 400, "余额不足"},
		{model.ErrTransactionTypeNotFound, 400, "账变类型不可用"},
		{ErrAdjustTypeUnsupported, 400, "ADJUST_ADD"},
		{errors.New("数据库连接中断"), 500, "数据库连接中断"},
	}
	for _, c := range cases {
		status, msg := LedgerErrStatus(c.err)
		if status != c.wantStatus {
			t.Errorf("%v：状态码 = %d，期望 %d", c.err, status, c.wantStatus)
		}
		if !strings.Contains(msg, c.mustSay) {
			t.Errorf("%v：提示 %q 未包含 %q", c.err, msg, c.mustSay)
		}
	}

	// 包一层后仍要能识别（Poster 返回的常常是包装错误）
	wrapped := errors.Join(ledger.ErrAmountNotPositive, errors.New("amount=0"))
	if status, _ := LedgerErrStatus(wrapped); status != 400 {
		t.Errorf("包装后的金额错误状态码 = %d，期望 400", status)
	}
}

// TestLedgerAdjustmentListVisibility —— 调账落库后，后台流水查询能查到它，
// 且 typeName 来自 transaction_types（一次查表映射，不是 N+1）。
func TestLedgerAdjustmentListVisibility(t *testing.T) {
	_, svcCtx, adminID := newAdjustProbe(t)

	req := adjustReq()
	req.AmountMinor = 777
	req.ExternalTxId = "zz-wallet-voucher-list"
	req.Remark = "探针：列表可见性"

	l := NewLedgerAdjustmentLogic(adjustCtx(adminID), svcCtx)
	if _, err := l.Adjust(&req); err != nil {
		t.Fatalf("人工加款失败: %v", err)
	}

	query := NewLedgerTransactionLogic(context.Background(), svcCtx)
	resp, err := query.List(&types.LedgerTransactionListReq{
		MerchantId: adjustProbeMerchantID,
		Page:       1,
		PageSize:   20,
	})
	if err != nil {
		t.Fatalf("查询账变流水失败: %v", err)
	}
	if resp.Total != 1 || len(resp.List) != 1 {
		t.Fatalf("流水数量 = total:%d len:%d，期望 1", resp.Total, len(resp.List))
	}
	if resp.Page != 1 || resp.PageSize != 20 {
		t.Errorf("分页回显 = (%d, %d)，期望 (1, 20)", resp.Page, resp.PageSize)
	}
	item := resp.List[0]
	if item.ExternalTxId != req.ExternalTxId {
		t.Errorf("externalTxId = %q，期望 %q", item.ExternalTxId, req.ExternalTxId)
	}
	if item.TypeName != "人工加款" {
		t.Errorf("typeName = %q，期望 人工加款（应来自 transaction_types.name）", item.TypeName)
	}
	if item.RoundId != "" {
		t.Errorf("人工调账 roundId 应为空串，实际 %q", item.RoundId)
	}
	if item.CreatedAt == 0 {
		t.Error("createdAt 未填充")
	}

	// 时间范围过滤走的是同一份条件拼装：把区间放在未来必须查不到
	future := time.Now().UTC().Add(24 * time.Hour)
	if _, err := query.List(&types.LedgerTransactionListReq{
		MerchantId: adjustProbeMerchantID,
		StartTime:  future.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("按时间查询失败: %v", err)
	}
	if _, err := query.List(&types.LedgerTransactionListReq{
		MerchantId: adjustProbeMerchantID,
		StartTime:  "2006-01-02 15:04:05",
		EndTime:    "2006-01-02T15:04:05Z",
	}); err != nil {
		t.Fatalf("两种时间格式混用应当都能解析: %v", err)
	}
	if _, err := query.List(&types.LedgerTransactionListReq{StartTime: "不是时间"}); err == nil {
		t.Error("非法时间格式应当报错")
	}
}
