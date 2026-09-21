package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrDuplicateEvent = errors.New("outbox: duplicate event")

type Store struct {
	conn Executor
}

func NewStore(conn Executor) *Store {
	return &Store{conn: conn}
}

const insertRecordSQL = `
INSERT INTO event_outbox
  (id, topic, merchant_id, partition_key, payload, status, delivery_mode, retry_count, next_retry_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`

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

func (s *Store) Insert(ctx context.Context, rec *Record) error {
	return s.InsertInTx(ctx, s.conn, rec)
}

const claimSQL = `
SELECT id, topic, IFNULL(merchant_id,0), partition_key, payload, delivery_mode, retry_count
FROM event_outbox
WHERE status = ? AND next_retry_at <= ?
ORDER BY id
LIMIT ?
FOR UPDATE SKIP LOCKED`

// ClaimBatch 批量领取待投递记录并一次性翻转为 IN_FLIGHT
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

	ids := make([]any, len(rows))
	placeholders := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		placeholders[i] = "?"
	}

	// 优化：将原本 N 次逐条 UPDATE 聚合为 1 次批量更新，消除网络 RTT 瓶颈
	batchMarkSQL := fmt.Sprintf(`
		UPDATE event_outbox
		SET status = ?, claim_owner = ?, claim_at = ?
		WHERE id IN (%s) AND status = ?
	`, strings.Join(placeholders, ","))

	args := append([]any{StatusInFlight, owner, now}, ids...)
	args = append(args, StatusPending)

	if _, err := tx.ExecCtx(ctx, batchMarkSQL, args...); err != nil {
		return nil, fmt.Errorf("outbox: batch mark in-flight: %w", err)
	}

	return rows, nil
}

const markSentBatchSQL = `
UPDATE event_outbox
SET status = ?, sent_at = ?, last_error = NULL, claim_owner = NULL
WHERE status = ? AND claim_owner = ? AND id IN (`

const markRetrySQL = `
UPDATE event_outbox
SET status = ?, retry_count = retry_count + 1, next_retry_at = ?, last_error = ?,
    claim_owner = NULL, claim_at = NULL
WHERE id = ? AND status = ? AND claim_owner = ?`

const markFailedSQL = `
UPDATE event_outbox
SET status = ?, last_error = ?, claim_owner = NULL, claim_at = NULL
WHERE id = ? AND status = ? AND claim_owner = ?`

// SettleBatch 批量结算投递结果
func (s *Store) SettleBatch(ctx context.Context, owner string, records []*Record, results []error, backoff Backoff) ([]string, error) {
	if len(records) != len(results) {
		return nil, fmt.Errorf("outbox: settle length mismatch: %d records vs %d results", len(records), len(results))
	}
	now := time.Now().UTC()
	var failed []string

	var successIDs []any
	var successPlaceholders []string

	for i, rec := range records {
		if results[i] == nil {
			successIDs = append(successIDs, rec.ID)
			successPlaceholders = append(successPlaceholders, "?")
			continue
		}

		// 失败记录单独按退避策略更新
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

	// 优化：针对批量投递成功的记录，单次 SQL 批量打标 SENT
	if len(successIDs) > 0 {
		query := markSentBatchSQL + strings.Join(successPlaceholders, ",") + ")"
		args := append([]any{StatusSent, now, StatusInFlight, owner}, successIDs...)
		if _, err := s.conn.ExecCtx(ctx, query, args...); err != nil {
			return failed, fmt.Errorf("outbox: batch mark sent: %w", err)
		}
	}

	return failed, nil
}

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

type Idempotency struct {
	conn Executor
}

func NewIdempotency(conn Executor) *Idempotency {
	return &Idempotency{conn: conn}
}

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
	return n > 0, nil
}

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
