package handler

// ledgerHandler_test.go —— 账变两个接口的 HTTP 层冒烟测试（需要 fastgame-mysql 在线）
//
// logic 层的行为已由 services/admin/internal/logic 的测试覆盖，这里只验证
// 接口契约：JSON/form 标签能被 httpx.Parse 正确解析（driftOnly 布尔、分页默认值、
// 时间字符串），以及错误被映射成预期的状态码与 {"message": ...} 结构。
//
// MySQL 不可达时自动跳过；探针用 merchant_id = 990003，用完清掉自己的数据。

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/fieldcipher"
	"fastgame/pkg/ledger"
	"fastgame/services/admin/internal/svc"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	defaultHandlerTestDSN  = "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"
	handlerProbeMerchantID = 990003
	// handlerProbeAdminID 放进上下文里的管理员 id，只用于让 logic 去查用户名。
	handlerProbeAdminID = uint64(4242)
)

func newLedgerHandlerCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = defaultHandlerTestDSN
	}
	fieldcipher.Init("")

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

	cleanup := func() {
		db.Exec("DELETE FROM game_transactions WHERE merchant_id = ?", handlerProbeMerchantID)
		db.Exec("DELETE FROM player_accounts WHERE merchant_id = ?", handlerProbeMerchantID)
	}
	cleanup()
	t.Cleanup(func() {
		cleanup()
		db.Close()
	})

	// 操作人沿用 admin_users 里的任意一行（FindOne 只用于取 username，不会写库）。
	var adminID uint64
	if err := db.QueryRow("SELECT id FROM admin_users ORDER BY id LIMIT 1").Scan(&adminID); err != nil {
		t.Skipf("跳过：admin_users 为空（%v）", err)
	}

	conn := sqlx.NewMysql(dsn)
	return &svc.ServiceContext{
		AdminUsers:  model.NewAdminUsersModel(conn),
		Ledger:      ledger.NewPoster(conn, nil),
		LedgerModel: model.NewLedgerModel(),
		DB:          conn,
	}
}

// serveAdjustment 直接调用 handler（不带中间件）：中间件链路由 routes.go 统一挂载，
// 见 routes.go 里 ledger 两条路由所在的 AddRoutes 分组。
func serveAdjustment(t *testing.T, svcCtx *svc.ServiceContext, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/ledger/adjustments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.1.2.3:5678"
	req = req.WithContext(context.WithValue(req.Context(), "userId", float64(handlerProbeAdminID)))

	rec := httptest.NewRecorder()
	LedgerAdjustmentHandler(svcCtx)(rec, req)
	return rec
}

// TestLedgerAdjustmentHandlerContract —— 请求体解析 + 响应体字段 + 错误状态码。
func TestLedgerAdjustmentHandlerContract(t *testing.T) {
	svcCtx := newLedgerHandlerCtx(t)

	// 1) 正常路径
	body := `{"merchantId":990003,"userId":"zz-handler-probe","typeCode":"ADJUST_ADD",
	          "amountMinor":50000,"externalTxId":"zz-handler-voucher-1","remark":"接口冒烟：加款"}`
	rec := serveAdjustment(t, svcCtx, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200，body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (%s)", err, rec.Body.String())
	}
	for _, key := range []string{"transactionId", "txType", "direction", "amountMinor",
		"balanceBeforeMinor", "balanceAfterMinor", "status", "driftMinor", "duplicated"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("响应缺少字段 %s: %s", key, rec.Body.String())
		}
	}
	if resp["txType"] != "ADJUST_ADD" || resp["direction"] != "IN" || resp["status"] != "SUCCESS" {
		t.Errorf("txType/direction/status = %v/%v/%v", resp["txType"], resp["direction"], resp["status"])
	}
	if resp["balanceAfterMinor"].(float64) != 50000 {
		t.Errorf("balanceAfterMinor = %v，期望 50000", resp["balanceAfterMinor"])
	}

	// 2) 类型不在白名单 → 400 且 message 说明只支持 ADJUST_ADD/ADJUST_SUB
	bad := `{"merchantId":990003,"userId":"zz-handler-probe","typeCode":"BET",
	         "amountMinor":100,"externalTxId":"zz-handler-voucher-2","remark":"接口冒烟：非法类型"}`
	rec = serveAdjustment(t, svcCtx, bad)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法类型状态码 = %d，期望 400，body=%s", rec.Code, rec.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("错误响应不是合法 JSON: %v (%s)", err, rec.Body.String())
	}
	if errResp["message"] == "" {
		t.Errorf("错误响应缺少 message: %s", rec.Body.String())
	}

	// 3) 金额非正 → 400 且 message 明确说"金额必须为正数"
	zero := `{"merchantId":990003,"userId":"zz-handler-probe","typeCode":"ADJUST_ADD",
	          "amountMinor":0,"externalTxId":"zz-handler-voucher-3","remark":"接口冒烟：金额为 0"}`
	rec = serveAdjustment(t, svcCtx, zero)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("金额为 0 状态码 = %d，期望 400，body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("错误响应不是合法 JSON: %v", err)
	}
	if !bytes.Contains([]byte(errResp["message"]), []byte("金额必须为正数")) {
		t.Errorf("message = %q，期望包含 金额必须为正数", errResp["message"])
	}
}

// TestLedgerTransactionListHandlerContract —— query 参数解析（含 bool/分页/时间）
// 与 {list,total,page,pageSize} 响应结构。
func TestLedgerTransactionListHandlerContract(t *testing.T) {
	svcCtx := newLedgerHandlerCtx(t)

	// 先造一条流水，保证 list 非空
	body := `{"merchantId":990003,"userId":"zz-handler-probe","typeCode":"ADJUST_ADD",
	          "amountMinor":31400,"externalTxId":"zz-handler-voucher-list","remark":"接口冒烟：列表"}`
	if rec := serveAdjustment(t, svcCtx, body); rec.Code != http.StatusOK {
		t.Fatalf("准备数据失败：状态码 %d，body=%s", rec.Code, rec.Body.String())
	}

	// 时间参数里带空格，必须转义后再拼进 query string
	start := url.QueryEscape(time.Now().UTC().Add(-time.Hour).Format("2006-01-02 15:04:05"))
	end := url.QueryEscape(time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/ledger/transactions?merchantId=990003&driftOnly=false&page=1&pageSize=10"+
			"&startTime="+start+"&endTime="+end, nil)
	rec := httptest.NewRecorder()
	LedgerTransactionListHandler(svcCtx)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200，body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		List     []map[string]any `json:"list"`
		Total    int64            `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"pageSize"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (%s)", err, rec.Body.String())
	}
	if resp.Total != 1 || len(resp.List) != 1 {
		t.Fatalf("total/len = %d/%d，期望 1/1，body=%s", resp.Total, len(resp.List), rec.Body.String())
	}
	if resp.Page != 1 || resp.PageSize != 10 {
		t.Errorf("分页回显 = (%d, %d)，期望 (1, 10)", resp.Page, resp.PageSize)
	}
	item := resp.List[0]
	for _, key := range []string{"id", "transactionId", "externalTxId", "txType", "direction",
		"merchantId", "merchantCode", "userId", "gameCode", "roundId", "currency", "amount",
		"balanceBefore", "balanceAfter", "driftMinor", "status", "remark", "typeName", "createdAt"} {
		if _, ok := item[key]; !ok {
			t.Errorf("列表项缺少字段 %s: %v", key, item)
		}
	}
	if item["typeName"] != "人工加款" {
		t.Errorf("typeName = %v，期望 人工加款", item["typeName"])
	}
	if item["externalTxId"] != "zz-handler-voucher-list" {
		t.Errorf("externalTxId = %v", item["externalTxId"])
	}
	if item["roundId"] != "" {
		t.Errorf("roundId = %v，期望空串", item["roundId"])
	}
	if item["createdAt"].(float64) <= 0 {
		t.Errorf("createdAt = %v，期望 unix 秒", item["createdAt"])
	}

	// 不传参数时 page/pageSize 走默认值 1/20
	rec = httptest.NewRecorder()
	LedgerTransactionListHandler(svcCtx)(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/ledger/transactions", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("默认分页响应不是合法 JSON: %v (%s)", err, rec.Body.String())
	}
	if resp.Page != 1 || resp.PageSize != 20 {
		t.Errorf("默认分页 = (%d, %d)，期望 (1, 20)", resp.Page, resp.PageSize)
	}

	// 非法时间格式应当被拒绝（不允许静默忽略过滤条件）
	rec = httptest.NewRecorder()
	LedgerTransactionListHandler(svcCtx)(rec, httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/ledger/transactions?startTime=not-a-time", nil))
	if rec.Code == http.StatusOK {
		t.Errorf("非法 startTime 应当报错，实际 200: %s", rec.Body.String())
	}
}
