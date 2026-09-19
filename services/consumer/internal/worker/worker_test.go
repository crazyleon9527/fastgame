package worker

// 需要真实 MySQL（docker/mysql/init/32-outbox-migration.sql）。
// MySQL 不可达时整组 skip，不影响常规 `go test ./...`。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/kafka"
	"fastgame/pkg/outbox"
	"fastgame/services/consumer/internal/config"
	"fastgame/services/consumer/internal/svc"
	_ "github.com/go-sql-driver/mysql"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const testDSN = "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"

func testWorker(t *testing.T, requireEnvelope bool) (*Worker, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = testDSN
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("跳过：%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("跳过：MySQL 不可达（%v）", err)
	}

	conn := sqlx.NewMysql(dsn)
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Kafka: config.KafkaConf{RequireEnvelope: requireEnvelope},
		},
		Merchants: model.NewMerchantsModel(conn),
		DB:        conn,
		Idem:      outbox.NewIdempotency(conn),
	}
	return &Worker{svcCtx: svcCtx}, db
}

// ---------------------------------------------------------------------------
// 解析：必须同时兼容 outbox 信封与旧的裸事件格式
// ---------------------------------------------------------------------------

func TestParseMessageEnvelopeAndBare(t *testing.T) {
	w, db := testWorker(t, false)
	t.Cleanup(func() { db.Close() })

	evt := kafka.RoundSettledEvent{
		EventID:    "01a0b67a-0000-7000-8000-000000000001",
		TraceID:    "trace-abc",
		RoundID:    "round-1",
		UserID:     "U-parse",
		MerchantID: "m001",
		GameCode:   "fishing_tycoon",
		BetAmount:  10000,
		WinAmount:  15000,
		Multiplier: 15000,
		RtpTier:    "default",
		Balance:    100000,
		SettledAt:  time.Now().UTC(),
	}

	// 1) 新版：outbox 信封
	env, err := outbox.NewEnvelopeWithID(outbox.EventTopic(kafka.TopicRoundSettled), "rgs-api",
		evt.TraceID, evt.EventID, evt, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	row, parsed, err := w.parseMessage(env)
	if err != nil {
		t.Fatalf("解析信封失败: %v", err)
	}
	if parsed.RoundID != "round-1" {
		t.Fatalf("信封内 roundId 未解析出来，得到 %q", parsed.RoundID)
	}
	if parsed.UserID != "U-parse" {
		t.Fatalf("信封内 userId 未解析出来，得到 %q", parsed.UserID)
	}
	if row.EventID != evt.EventID {
		t.Fatalf("eventId 应透传，期望 %q 得到 %q", evt.EventID, row.EventID)
	}
	if row.MerchantID == 0 {
		t.Fatal("merchantCode 应被解析成 merchant_id")
	}
	t.Log("✓ 信封格式解析正确（含 payload 解包与 merchant 解析）")

	// 2) 旧版：裸事件 JSON（滚动升级兼容）
	bare, _ := json.Marshal(evt)
	row2, parsed2, err := w.parseMessage(bare)
	if err != nil {
		t.Fatalf("解析裸事件失败: %v", err)
	}
	if parsed2.RoundID != "round-1" || row2.UserID != "U-parse" {
		t.Fatalf("裸格式解析异常: round=%q user=%q", parsed2.RoundID, row2.UserID)
	}
	t.Log("✓ 裸事件格式仍兼容（滚动升级不会丢消息）")

	// 3) RequireEnvelope=true 时拒绝裸格式
	w2, db2 := testWorker(t, true)
	defer db2.Close()
	if _, _, err := w2.parseMessage(bare); err == nil {
		t.Fatal("RequireEnvelope=true 时应拒绝裸格式消息")
	}
	t.Log("✓ RequireEnvelope=true 时裸格式被拒绝")
}

// ---------------------------------------------------------------------------
// 幂等：同一 eventID 第二次必须被拦住（at-least-once 的配套）
// ---------------------------------------------------------------------------

func TestClaimIdempotencyBlocksDuplicate(t *testing.T) {
	w, db := testWorker(t, false)
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	eventID := "01a0b67a-0000-7000-8000-0000000000ff"
	db.Exec("DELETE FROM processed_events WHERE consumer_group = ? AND event_id = ?", consumerGroup, eventID)
	t.Cleanup(func() {
		db.Exec("DELETE FROM processed_events WHERE consumer_group = ? AND event_id = ?", consumerGroup, eventID)
	})

	if !w.claimIdempotency(ctx, eventID) {
		t.Fatal("首次应认领成功")
	}
	if w.claimIdempotency(ctx, eventID) {
		t.Fatal("重复事件应被拦住（否则 ClickHouse 会重复计账）")
	}
	t.Log("✓ 重复 eventID 被幂等拦住")

	// 空 eventID（旧格式消息）不做去重，应放行
	if !w.claimIdempotency(ctx, "") {
		t.Fatal("空 eventID 应放行")
	}
	t.Log("✓ 空 eventID 放行（交由 ClickHouse 侧兜底）")
}

// ---------------------------------------------------------------------------
// 幂等占位在事务回滚时必须消失（幽灵占位行的回归防线）
// ---------------------------------------------------------------------------

func TestClaimIdempotencyNoGhostRowOnRollback(t *testing.T) {
	w, db := testWorker(t, false)
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	eventID := "01a0b67a-0000-7000-8000-0000000000ee"
	db.Exec("DELETE FROM processed_events WHERE consumer_group = ? AND event_id = ?", consumerGroup, eventID)
	t.Cleanup(func() {
		db.Exec("DELETE FROM processed_events WHERE consumer_group = ? AND event_id = ?", consumerGroup, eventID)
	})

	sentinel := errors.New("clickhouse write failed")
	err := w.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		ok, err := w.svcCtx.Idem.AcquireInTx(ctx, session, consumerGroup, eventID)
		if err != nil {
			return err
		}
		if !ok {
			return outbox.ErrDuplicateEvent
		}
		return sentinel // 模拟落库失败 → 回滚
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("期望业务错误，实际 %v", err)
	}

	var n int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM processed_events WHERE consumer_group = ? AND event_id = ?",
		consumerGroup, eventID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("落库失败回滚后幂等占位仍在 —— 会导致该事件被永久跳过")
	}
	// 槽位应可重新认领（消息重投后能正常处理）
	if !w.claimIdempotency(ctx, eventID) {
		t.Fatal("回滚后应可重新认领")
	}
	t.Log("✓ 落库失败时幂等占位随事务回滚，重投可正常处理")
}
