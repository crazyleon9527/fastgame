package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type WalletRollbackRow struct {
	EventID      string
	TraceID      string
	RoundID      string
	UserID       uint64
	MerchantID   uint64
	RollbackType string
	Amount       float64
	Reason       string
	Status       string
	OccurredAt   time.Time
}

type TraceSpanRow struct {
	TraceID    string
	SpanID     string
	Service    string
	Operation  string
	RoundID    string
	Status     string
	Detail     string
	DurationMs uint32
	OccurredAt time.Time
}

type RoundSettledRow struct {
	EventID      string
	TraceID      string
	RoundID      string
	UserID       uint64
	MerchantID   uint64
	GameCode     string
	BetAmount    float64
	WinAmount    float64
	Multiplier   float64
	RtpTier      string
	BalanceAfter float64
	SettledAt    time.Time
}

type Writer struct {
	conn driver.Conn
}

func NewWriter(dsn string) (*Writer, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
		Auth: clickhouse.Auth{
			Database: "fastgame",
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	return &Writer{conn: conn}, nil
}

func NewWriterWithAuth(addr, database, user, password string) (*Writer, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: user,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	return &Writer{conn: conn}, nil
}

func (w *Writer) BatchInsertRoundSettled(ctx context.Context, rows []RoundSettledRow) error {
	if len(rows) == 0 {
		return nil
	}

	batch, err := w.conn.PrepareBatch(ctx, `
		INSERT INTO fastgame.game_round_settled (
			event_id, trace_id, round_id, user_id, merchant_id, game_code,
			bet_amount, win_amount, multiplier, rtp_tier, balance_after, settled_at
		)
	`)
	if err != nil {
		return err
	}

	for _, row := range rows {
		eventID, err := uuid.Parse(row.EventID)
		if err != nil {
			eventID = uuid.New()
		}
		settledAt := row.SettledAt
		if settledAt.IsZero() {
			settledAt = time.Now().UTC()
		}
		if err := batch.Append(
			eventID,
			row.TraceID,
			row.RoundID,
			row.UserID,
			row.MerchantID,
			row.GameCode,
			decimal.NewFromFloat(row.BetAmount),
			decimal.NewFromFloat(row.WinAmount),
			decimal.NewFromFloat(row.Multiplier),
			row.RtpTier,
			decimal.NewFromFloat(row.BalanceAfter),
			settledAt,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (w *Writer) BatchInsertWalletRollback(ctx context.Context, rows []WalletRollbackRow) error {
	if len(rows) == 0 {
		return nil
	}

	batch, err := w.conn.PrepareBatch(ctx, `
		INSERT INTO fastgame.game_wallet_rollback (
			event_id, trace_id, round_id, user_id, merchant_id, rollback_type,
			amount, reason, status, occurred_at
		)
	`)
	if err != nil {
		return err
	}

	for _, row := range rows {
		eventID, err := uuid.Parse(row.EventID)
		if err != nil {
			eventID = uuid.New()
		}
		occurredAt := row.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}
		status := row.Status
		if status == "" {
			status = "done"
		}
		if err := batch.Append(
			eventID,
			row.TraceID,
			row.RoundID,
			row.UserID,
			row.MerchantID,
			row.RollbackType,
			decimal.NewFromFloat(row.Amount),
			row.Reason,
			status,
			occurredAt,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (w *Writer) BatchInsertTraceSpans(ctx context.Context, rows []TraceSpanRow) error {
	if len(rows) == 0 {
		return nil
	}

	batch, err := w.conn.PrepareBatch(ctx, `
		INSERT INTO fastgame.trace_spans (
			trace_id, span_id, service, operation, round_id, status, detail, duration_ms, occurred_at
		)
	`)
	if err != nil {
		return err
	}

	for _, row := range rows {
		occurredAt := row.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}
		if err := batch.Append(
			row.TraceID,
			row.SpanID,
			row.Service,
			row.Operation,
			row.RoundID,
			row.Status,
			row.Detail,
			row.DurationMs,
			occurredAt,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (w *Writer) QueryTraceSpans(ctx context.Context, traceID string) ([]TraceSpanRow, error) {
	rows, err := w.conn.Query(ctx, `
		SELECT trace_id, span_id, service, operation, round_id, status, detail, duration_ms, occurred_at
		FROM fastgame.trace_spans
		WHERE trace_id = ?
		ORDER BY occurred_at ASC
		LIMIT 500
	`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TraceSpanRow
	for rows.Next() {
		var row TraceSpanRow
		if err := rows.Scan(
			&row.TraceID, &row.SpanID, &row.Service, &row.Operation, &row.RoundID,
			&row.Status, &row.Detail, &row.DurationMs, &row.OccurredAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (w *Writer) Close() error {
	return w.conn.Close()
}
