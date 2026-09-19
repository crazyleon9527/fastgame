package outbox

// 需要真实 MySQL（应用 docker/mysql/init/32-outbox-migration.sql）。
// 未设置 MYSQL_DSN 且本地库不可达时自动 skip，因此不影响常规 `go test ./...`。
//
// 运行：
//   go test ./pkg/outbox/ -v

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const defaultDSN = "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"

func dsn() string {
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		return v
	}
	return defaultDSN
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn())
	if err != nil {
		t.Skipf("跳过：无法创建连接（%v）", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("跳过：MySQL 不可达（%v）", err)
	}
	return db
}

func newConn(t *testing.T) sqlx.SqlConn {
	t.Helper()
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	return sqlx.NewMysql(dsn())
}

// recordingDeliverer 记录投递过的记录，可配置为全部失败。
type recordingDeliverer struct {
	got      []*Record
	failWith error
}

func (d *recordingDeliverer) Deliver(_ context.Context, records []*Record) []error {
	errs := make([]error, len(records))
	for i, r := range records {
		d.got = append(d.got, r)
		if d.failWith != nil {
			errs[i] = d.failWith
		}
	}
	return errs
}

func countByStatus(t *testing.T, db *sql.DB, status int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM event_outbox WHERE status = ?", status).Scan(&n); err != nil {
		t.Fatalf("count status=%d: %v", status, err)
	}
	return n
}

// isolatePending 把表里遗留的 PENDING 行挪走，保证本用例的目标记录一定落在
// ClaimBatch 的 limit 之内。
//
// 为什么需要：ClaimBatch 是 `ORDER BY id LIMIT n`，同一包内先跑的用例若留下
// 待投递行（例如故意制造失败场景），会把 limit 占满，导致本用例的记录根本
// 没被领取——单独跑通过、整包跑失败，属于典型的测试相互干扰。
//
// 用置为 FAILED 而非 DELETE：FAILED 是终态不会被领取，且顺带覆盖了
// "FAILED 记录不再参与派发"这条语义。
func isolatePending(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		"UPDATE event_outbox SET status = ? WHERE status = ?", StatusFailed, StatusPending); err != nil {
		t.Fatalf("isolate pending: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 1. 事务性：业务写回滚时，事件登记必须一起回滚（核心不变量）
// ---------------------------------------------------------------------------

func TestOutboxRollsBackWithBusinessTx(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	store := NewStore(conn)
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{"probe": "rollback"})

	// 故意让事务回滚
	sentinel := errors.New("business failed")
	err := conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := store.InsertInTx(ctx, session, &Record{
			Topic:        "game.round.settled",
			PartitionKey: "U-rollback",
			Payload:      payload,
		}); err != nil {
			return err
		}
		return sentinel // 业务失败 → 整个事务回滚
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("期望业务错误透出，实际 %v", err)
	}

	var n int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM event_outbox WHERE partition_key = ?", "U-rollback").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("事务回滚后事件仍在表里（count=%d）—— 事务性未生效", n)
	}
	t.Log("✓ 业务事务回滚时事件登记一并回滚")
}

// ---------------------------------------------------------------------------
// 2. 正常投递：Claim → Deliver → Settle 后状态为 SENT
// ---------------------------------------------------------------------------

func TestOutboxClaimDeliverSettle(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	store := NewStore(conn)
	ctx := context.Background()
	isolatePending(t, db)

	key := fmt.Sprintf("U-sent-%d", time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]string{"probe": "sent"})
	rec := &Record{Topic: "game.round.settled", PartitionKey: key, Payload: payload}
	if err := store.Insert(ctx, rec); err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM event_outbox WHERE id = ?", rec.ID) })

	d := &recordingDeliverer{}
	disp := NewDispatcher(conn, d, DispatcherConfig{Owner: "test-sent", BatchSize: 10})

	n, err := disp.DispatchOnce(ctx)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if n < 1 {
		t.Fatalf("期望至少处理 1 条，实际 %d", n)
	}
	if len(d.got) == 0 {
		t.Fatal("deliverer 未收到记录")
	}

	var status int64
	if err := db.QueryRow("SELECT status FROM event_outbox WHERE id = ?", rec.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusSent {
		t.Fatalf("期望 status=%d(SENT)，实际 %d", StatusSent, status)
	}
	t.Log("✓ 投递成功后状态置为 SENT")
}

// ---------------------------------------------------------------------------
// 3. 投递失败：状态回到 PENDING 且带退避，超过上限才置 FAILED
// ---------------------------------------------------------------------------

func TestOutboxRetryThenFail(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	store := NewStore(conn)
	ctx := context.Background()
	isolatePending(t, db)

	key := fmt.Sprintf("U-retry-%d", time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]string{"probe": "retry"})
	rec := &Record{Topic: "game.round.settled", PartitionKey: key, Payload: payload}
	if err := store.Insert(ctx, rec); err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM event_outbox WHERE id = ?", rec.ID) })

	// 用极短退避以便快速走到 FAILED
	d := &recordingDeliverer{failWith: errors.New("simulated kafka outage")}
	disp := NewDispatcher(conn, d, DispatcherConfig{Owner: "test-retry", BatchSize: 10})
	disp.backoff = NewBackoff(time.Millisecond, time.Millisecond)

	// 第一次：PENDING → 重试一次后仍是 PENDING（并写入 next_retry_at）
	if _, err := disp.DispatchOnce(ctx); err != nil {
		t.Fatalf("dispatch 1: %v", err)
	}
	var status, retry int64
	if err := db.QueryRow("SELECT status, retry_count FROM event_outbox WHERE id = ?", rec.ID).
		Scan(&status, &retry); err != nil {
		t.Fatal(err)
	}
	if status != StatusPending {
		t.Fatalf("失败一次后应回到 PENDING(0)，实际 %d", status)
	}
	if retry != 1 {
		t.Fatalf("retry_count 应为 1，实际 %d", retry)
	}

	// 再跑一轮（退避 1ms 已过）：达到 MaxRetries(2) → FAILED
	time.Sleep(5 * time.Millisecond)
	if _, err := disp.DispatchOnce(ctx); err != nil {
		t.Fatalf("dispatch 2: %v", err)
	}
	if err := db.QueryRow("SELECT status FROM event_outbox WHERE id = ?", rec.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusFailed {
		t.Fatalf("超过最大重试后应置 FAILED(%d)，实际 %d", StatusFailed, status)
	}
	t.Log("✓ 失败退避重试，超过上限置 FAILED 并留痕")
}

// ---------------------------------------------------------------------------
// 4. 幂等：AcquireInTx 首次占用成功、重复占用失败
// ---------------------------------------------------------------------------

func TestIdempotencyAcquireInTx(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	idem := NewIdempotency(conn)
	ctx := context.Background()

	eventID, err := NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	group := "test-group"
	t.Cleanup(func() {
		db.Exec("DELETE FROM processed_events WHERE consumer_group = ? AND event_id = ?", group, eventID)
	})

	// 第一次：占用成功
	var first bool
	if err := conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var e error
		first, e = idem.AcquireInTx(ctx, session, group, eventID)
		return e
	}); err != nil {
		t.Fatalf("第一次 acquire: %v", err)
	}
	if !first {
		t.Fatal("首次占用应返回 true")
	}

	// 第二次：应返回 false
	var second bool
	if err := conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var e error
		second, e = idem.AcquireInTx(ctx, session, group, eventID)
		return e
	}); err != nil {
		t.Fatalf("第二次 acquire: %v", err)
	}
	if second {
		t.Fatal("重复事件应返回 false（已处理过）")
	}
	t.Log("✓ 同一 eventID 首次占用成功、重复占用被识别")
}

// TestIdempotencyRollbackReleasesSlot 是"幽灵占位行"问题的回归测试：
// 业务事务回滚时，幂等占位行必须一起消失，否则下次重投会被错误 ACK，
// 事件永久丢失。这正是 platform-api 把旧 wrapper 式 API 标 Deprecated 的原因。
func TestIdempotencyRollbackReleasesSlot(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	idem := NewIdempotency(conn)
	ctx := context.Background()

	eventID, err := NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	group := "test-group-rb"
	t.Cleanup(func() {
		db.Exec("DELETE FROM processed_events WHERE consumer_group = ? AND event_id = ?", group, eventID)
	})

	sentinel := errors.New("business write failed")
	err = conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		ok, e := idem.AcquireInTx(ctx, session, group, eventID)
		if e != nil {
			return e
		}
		if !ok {
			return ErrDuplicateEvent
		}
		return sentinel // 业务失败 → 回滚
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("期望业务错误，实际 %v", err)
	}

	var n int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM processed_events WHERE consumer_group = ? AND event_id = ?",
		group, eventID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("事务回滚后幂等占位仍在（count=%d）—— 会形成幽灵占位行导致事件被错误 ACK", n)
	}

	// 证明槽位确实可复用（下次重投能正常进入业务）
	var again bool
	if err := conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var e error
		again, e = idem.AcquireInTx(ctx, session, group, eventID)
		return e
	}); err != nil {
		t.Fatalf("重投 acquire: %v", err)
	}
	if !again {
		t.Fatal("回滚后槽位应可重新占用")
	}
	t.Log("✓ 业务回滚时幂等占位一并回滚，重投可正常处理")
}

// ---------------------------------------------------------------------------
// 5. Reaper：悬挂的 IN_FLIGHT 被回收成 PENDING
// ---------------------------------------------------------------------------

func TestReapStuckInFlight(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	store := NewStore(conn)
	ctx := context.Background()

	id, err := NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"probe": "stuck"})
	// 直接构造一条"领取后崩溃"的悬挂记录
	if _, err := db.Exec(`
INSERT INTO event_outbox (id, topic, partition_key, payload, status, delivery_mode, next_retry_at, claim_owner, claim_at)
VALUES (?, 'game.round.settled', 'U-stuck', ?, ?, 1, ?, 'dead-instance', ?)`,
		id, string(payload), StatusInFlight, time.Now().UTC().Add(-time.Hour),
		time.Now().UTC().Add(-10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM event_outbox WHERE id = ?", id) })

	n, err := store.ReapStuck(ctx, 2*time.Minute)
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if n < 1 {
		t.Fatalf("期望回收至少 1 条，实际 %d", n)
	}
	var status int64
	var owner sql.NullString
	if err := db.QueryRow("SELECT status, claim_owner FROM event_outbox WHERE id = ?", id).
		Scan(&status, &owner); err != nil {
		t.Fatal(err)
	}
	if status != StatusPending {
		t.Fatalf("回收后应为 PENDING，实际 %d", status)
	}
	if owner.Valid {
		t.Fatalf("回收后应清空 claim_owner，实际 %q", owner.String)
	}
	t.Log("✓ 悬挂 IN_FLIGHT 被回收为 PENDING，可重新投递")
}

// ---------------------------------------------------------------------------
// 6. SettleBatch 只按 status 计数，不会重复处理（幂等守卫）
// ---------------------------------------------------------------------------

func TestSettleOnlyMarksClaimedRows(t *testing.T) {
	db := openDB(t)
	t.Cleanup(func() { db.Close() })
	conn := newConn(t)
	store := NewStore(conn)
	ctx := context.Background()

	key := fmt.Sprintf("U-guard-%d", time.Now().UnixNano())
	payload, _ := json.Marshal(map[string]string{"probe": "guard"})
	rec := &Record{Topic: "game.round.settled", PartitionKey: key, Payload: payload}
	if err := store.Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM event_outbox WHERE id = ?", rec.ID) })

	// 用错误的 owner 结算：owner+status 守卫应使其不生效
	recs := []*Record{{ID: rec.ID, RetryCount: 0}}
	if _, err := store.SettleBatch(ctx, "wrong-owner", recs, []error{nil}, DefaultBackoff()); err != nil {
		t.Fatalf("settle: %v", err)
	}
	var status int64
	if err := db.QueryRow("SELECT status FROM event_outbox WHERE id = ?", rec.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusPending {
		t.Fatalf("owner 不匹配时不应改写状态，实际 %d", status)
	}
	t.Log("✓ SettleBatch 的 owner+status 双守卫生效，避免旧 owner 误标")
}

func countByStatusHelper(db *sql.DB, status int64) int {
	var n int
	_ = db.QueryRow("SELECT COUNT(*) FROM event_outbox WHERE status = ?", status).Scan(&n)
	return n
}
