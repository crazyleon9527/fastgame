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

type RoundSettledRow struct {
	EventID      string
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
			event_id, round_id, user_id, merchant_id, game_code,
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

func (w *Writer) Close() error {
	return w.conn.Close()
}
