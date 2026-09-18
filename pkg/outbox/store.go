package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ErrDuplicateEvent 是"控制流哨兵"：调用方在业务事务内调 AcquireInTx 拿到
// (false, nil) 后，应 return 本错误让事务 **回滚**（而非空提交，避免无意义的
// binlog），外层再用 errors.Is 把它过滤为 nil 以便向上游 ACK 该消息。
var ErrDuplicateEvent = errors.New("outbox: duplicate event")

// Store 发件箱与幂等表的读写。
type Store struct {
	conn Executor
}

// NewStore 传入 sqlx.SqlConn。
func NewStore(conn Executor) *Store {
	return &Store{conn: conn}
}

// ---------------------------------------------------------------- 写入侧

const insertRecordSQL = `
INSERT INTO event_outbox
  (id, topic, merchant_id, partition_key, payload, status, delivery_mode, retry_count, next_retry_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`

// InsertInTx 在调用方的事务里登记一条事件。
//
// 这是本包的核心 API：业务写与事件登记同事务提交，事务成功即事件不丢。
func (s *Store) InsertInTx(ctx context.Context, tx Executor, rec *Record) error {
	if rec.ID == "" {
		id, err := NewEventID()
		if err != nil {
			return err
		}
		rec.ID = id
	}
	if rec.DeliveryMode == 0 {
		rec.DeliveryMode = DeliveryKafka
	}
	var merchantID any
	if rec.MerchantID > 0 {
		merchantID = rec.MerchantID
	}
	_, err := tx.ExecCtx(ctx, insertRecordSQL,
		rec.ID, rec.Topic, merchantID, rec.PartitionKey,
		rawToDB(rec.Payload), StatusPending, rec.DeliveryMode, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("outbox: insert record: %w", err)
	}
	return nil
}

// Insert 独立登记一条事件（不参与别的事务，主要用于测试与后台补发）。
func (s *Store) Insert(ctx context.Context, rec *Record) error {
	return s.InsertInTx(ctx, s.conn, rec)
}

// ---------------------------------------------------------------- 派发侧

const claimSQL = `
SELECT id, topic, IFNULL(merchant_id,0), partition_key, payload, delivery_mode, retry_count
FROM event_outbox
WHERE status = ? AND next_retry_at <= ?
ORDER BY id
LIMIT ?
FOR UPDATE SKIP LOCKED`

const markInFlightSQL = `
UPDATE event_outbox
SET status = ?, claim_owner = ?, claim_at = ?
WHERE id = ? AND status = ?`

// ClaimBatch 领取一批待投递记录并翻成 IN_FLIGHT。
//
// 必须在事务内执行：SELECT ... FOR UPDATE SKIP LOCKED 让多个 dispatcher 实例
// 互不阻塞（SKIP LOCKED 跳过已被别人锁住的行），避免重复投递与锁等待。
func (s *Store) ClaimBatch(ctx context.Context, tx Executor, owner string, limit int) ([]*Record, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC()

	var rows []*Record
	if err := tx.QueryRowsCtx(ctx, &rows, claimSQL, StatusPending, now, limit); err != nil {
		return nil, fmt.Errorf("outbox: claim query: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	claimed := make([]*Record, 0, len(rows))
	for _, r := range rows {
		res, err := tx.ExecCtx(ctx, markInFlightSQL, StatusInFlight, owner, now, r.ID, StatusPending)
		if err != nil {
			return nil, fmt.Errorf("outbox: mark in-flight %s: %w", r.ID, err)
		}
		// RowsAffected == 0 说明这条已被其他实例领走，跳过即可（不算错误）
		n, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		claimed = append(claimed, r)
	}
	return claimed, nil
}

const markSentSQL = `
UPDATE event_outbox
SET status = ?, sent_at = ?, last_error = NULL, claim_owner = NULL
WHERE id = ? AND status = ? AND claim_owner = ?`

const markRetrySQL = `
UPDATE event_outbox
SET status = ?, retry_count = retry_count + 1, next_retry_at = ?, last_error = ?,
    claim_owner = NULL, claim_at = NULL
WHERE id = ? AND status = ? AND claim_owner = ?`

const markFailedSQL = `
UPDATE event_outbox
SET status = ?, last_error = ?, claim_owner = NULL, claim_at = NULL
WHERE id = ? AND status = ? AND claim_owner = ?`

// SettleBatch 按投递结果结算这一批。
//
// results 与 records 一一对应：nil 表示投递成功，非 nil 是该条的错误信息。
// 每次 UPDATE 都带 claim_owner + status 双守卫，防止"已被 Reaper 回收并重新
// 领取"的记录被旧 owner 错误标记（这正是 platform-api 注释里强调的幂等守卫）。
//
// 返回达到最大重试次数、被置为 FAILED 的记录 ID，供上层告警。
func (s *Store) SettleBatch(ctx context.Context, owner string, records []*Record, results []error, backoff Backoff) ([]string, error) {
	if len(records) != len(results) {
		return nil, fmt.Errorf("outbox: settle length mismatch: %d records vs %d results", len(records), len(results))
	}
	now := time.Now().UTC()
	var failed []string

	for i, rec := range records {
		if results[i] == nil {
			if _, err := s.conn.ExecCtx(ctx, markSentSQL, StatusSent, now, rec.ID, StatusInFlight, owner); err != nil {
				return failed, fmt.Errorf("outbox: mark sent %s: %w", rec.ID, err)
			}
			continue
		}

		msg := truncateErr(results[i].Error())
		nextRetry := int(rec.RetryCount) + 1
		if nextRetry >= backoff.MaxRetries() {
			if _, err := s.conn.ExecCtx(ctx, markFailedSQL, StatusFailed, msg, rec.ID, StatusInFlight, owner); err != nil {
				return failed, fmt.Errorf("outbox: mark failed %s: %w", rec.ID, err)
			}
			failed = append(failed, rec.ID)
			continue
		}
		if _, err := s.conn.ExecCtx(ctx, markRetrySQL, StatusPending, now.Add(backoff.Delay(nextRetry)), msg, rec.ID, StatusInFlight, owner); err != nil {
			return failed, fmt.Errorf("outbox: mark retry %s: %w", rec.ID, err)
		}
	}
	return failed, nil
}

// ReapStuck 回收悬挂的 IN_FLIGHT 记录（进程崩溃/投递卡死导致）。
// 超过 staleAfter 仍处于 IN_FLIGHT 的置回 PENDING，让其重新被领取。
func (s *Store) ReapStuck(ctx context.Context, staleAfter time.Duration) (int64, error) {
	res, err := s.conn.ExecCtx(ctx, `
UPDATE event_outbox
SET status = ?, claim_owner = NULL, claim_at = NULL, last_error = 'reaped: claim expired'
WHERE status = ? AND claim_at IS NOT NULL AND claim_at < ?`,
		StatusPending, StatusInFlight, time.Now().UTC().Add(-staleAfter))
	if err != nil {
		return 0, fmt.Errorf("outbox: reap stuck: %w", err)
	}
	return res.RowsAffected()
}

// PurgeSent 清理已投递成功且超过保留期的记录（分批）。
func (s *Store) PurgeSent(ctx context.Context, retain time.Duration, batch int) (int64, error) {
	if batch <= 0 {
		batch = 1000
	}
	res, err := s.conn.ExecCtx(ctx, `
DELETE FROM event_outbox
WHERE status = ? AND sent_at IS NOT NULL AND sent_at < ?
ORDER BY sent_at ASC
LIMIT ?`, StatusSent, time.Now().UTC().Add(-retain), batch)
	if err != nil {
		return 0, fmt.Errorf("outbox: purge sent: %w", err)
	}
	return res.RowsAffected()
}

// Stats 各状态计数，供运维观测（pending 持续增长说明投递有问题）。
func (s *Store) Stats(ctx context.Context) (map[int64]int64, error) {
	type row struct {
		Status int64 `db:"status"`
		N      int64 `db:"n"`
	}
	var rows []row
	if err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT status, COUNT(*) AS n FROM event_outbox GROUP BY status`); err != nil {
		return nil, fmt.Errorf("outbox: stats: %w", err)
	}
	out := make(map[int64]int64, len(rows))
	for _, r := range rows {
		out[r.Status] = r.N
	}
	return out, nil
}

// ---------------------------------------------------------------- 消费侧幂等

// Idempotency 消费端 DB 兜底幂等。
type Idempotency struct {
	conn Executor
}

func NewIdempotency(conn Executor) *Idempotency {
	return &Idempotency{conn: conn}
}

// AcquireInTx 在调用方**业务事务内**尝试占用 (consumerGroup, eventID) 槽位。
//
// 返回：
//   - (true,  nil)：首次占用，调用方继续业务写，随事务一起提交；
//   - (false, nil)：已处理过，调用方应 `return ErrDuplicateEvent` 让事务回滚，
//     外层用 errors.Is(err, ErrDuplicateEvent) 过滤为 nil 后 ACK 上游；
//   - (_,     err)：数据库抖动，调用方 return err 让事务回滚且**不 ACK**，
//     让上游重投。
//
// 典型用法：
//
//	err := conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
//	    ok, err := idem.AcquireInTx(ctx, session, group, eventID)
//	    if err != nil { return err }
//	    if !ok { return outbox.ErrDuplicateEvent }
//	    return businessWrite(ctx, session)
//	})
//	if errors.Is(err, outbox.ErrDuplicateEvent) { return nil } // 已处理，ACK
//	if err != nil { return err }                               // 不 ACK，等重投
func (i *Idempotency) AcquireInTx(ctx context.Context, tx Executor, consumerGroup, eventID string) (bool, error) {
	if consumerGroup == "" || eventID == "" {
		return false, fmt.Errorf("outbox: acquire needs consumerGroup and eventID")
	}
	res, err := tx.ExecCtx(ctx, `
INSERT INTO processed_events (consumer_group, event_id, processed_at)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE event_id = event_id`,
		consumerGroup, eventID, time.Now().UTC())
	if err != nil {
		return false, fmt.Errorf("outbox: acquire idempotency slot: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	// MySQL 的 ON DUPLICATE KEY UPDATE 在命中已有行时返回 0（未实际更新），
	// 因此 RowsAffected > 0 等价于"本次真正插入了占位行"。
	return n > 0, nil
}

// PurgeProcessed 清理超过保留期的幂等占位（分批，与 outbox GC 节奏一致）。
func (i *Idempotency) PurgeProcessed(ctx context.Context, retain time.Duration, batch int) (int64, error) {
	if batch <= 0 {
		batch = 1000
	}
	res, err := i.conn.ExecCtx(ctx, `
DELETE FROM processed_events
WHERE processed_at < ?
ORDER BY processed_at ASC
LIMIT ?`, time.Now().UTC().Add(-retain), batch)
	if err != nil {
		return 0, fmt.Errorf("outbox: purge processed events: %w", err)
	}
	return res.RowsAffected()
}

// EnvelopeFor 便利方法：组装信封并同时返回其 eventId。
func EnvelopeFor(topic EventTopic, source, traceID string, payload any, now time.Time) (raw json.RawMessage, eventID string, err error) {
	b, err := NewEnvelope(topic, source, traceID, payload, now)
	if err != nil {
		return nil, "", err
	}
	id, err := EnvelopeEventID(b)
	if err != nil {
		return nil, "", err
	}
	return b, id, nil
}

func truncateErr(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max]
}

var _ Executor = (sqlx.Session)(nil)
