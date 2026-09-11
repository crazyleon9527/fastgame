package clickhouse

import (
	"context"
	"fmt"
	"time"

	"fastgame/pkg/money"
)

type RtpReportRow struct {
	MerchantID  uint64
	GameCode    string
	Hour        time.Time
	TotalBet    float64
	TotalWin    float64
	TotalRounds uint64
	ActualRtp   float64
}

func (w *Writer) QueryRtpReport(ctx context.Context, merchantID uint64, gameCode string, hours int) ([]RtpReportRow, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	scale := money.Scale
	query := fmt.Sprintf(`
		SELECT
			merchant_id,
			game_code,
			hour,
			toFloat64(total_bet) / %d AS total_bet,
			toFloat64(total_win) / %d AS total_win,
			total_rounds,
			if(total_bet = 0, 0, toFloat64(total_win) / toFloat64(total_bet)) AS actual_rtp
		FROM fastgame.mv_rtp_hourly
		WHERE hour >= ?
	`, scale, scale)
	args := []any{since}
	if merchantID > 0 {
		query += " AND merchant_id = ?"
		args = append(args, merchantID)
	}
	if gameCode != "" {
		query += " AND game_code = ?"
		args = append(args, gameCode)
	}
	query += " ORDER BY hour DESC LIMIT 500"

	rows, err := w.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RtpReportRow
	for rows.Next() {
		var row RtpReportRow
		if err := rows.Scan(
			&row.MerchantID,
			&row.GameCode,
			&row.Hour,
			&row.TotalBet,
			&row.TotalWin,
			&row.TotalRounds,
			&row.ActualRtp,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

type DailySettlementRow struct {
	MerchantID  uint64
	SettleDate  time.Time
	TotalBet    int64
	TotalWin    int64
	TotalRounds uint64
}

func (w *Writer) QueryDailySettlement(ctx context.Context, merchantID uint64, days int) ([]DailySettlementRow, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().UTC().AddDate(0, 0, -days).Truncate(24 * time.Hour)

	query := `
		SELECT
			merchant_id,
			toDate(settled_at) AS settle_date,
			sum(bet_amount) AS total_bet,
			sum(win_amount) AS total_win,
			count() AS total_rounds
		FROM fastgame.game_round_settled
		WHERE settled_at >= ?
	`
	args := []any{since}
	if merchantID > 0 {
		query += " AND merchant_id = ?"
		args = append(args, merchantID)
	}
	query += " GROUP BY merchant_id, settle_date ORDER BY settle_date DESC, merchant_id ASC LIMIT 500"

	rows, err := w.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("daily settlement: %w", err)
	}
	defer rows.Close()

	var result []DailySettlementRow
	for rows.Next() {
		var row DailySettlementRow
		if err := rows.Scan(&row.MerchantID, &row.SettleDate, &row.TotalBet, &row.TotalWin, &row.TotalRounds); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// Fallback query when materialized view is empty.
func (w *Writer) QueryRtpReportFromRaw(ctx context.Context, merchantID uint64, gameCode string, hours int) ([]RtpReportRow, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	scale := money.Scale
	query := fmt.Sprintf(`
		SELECT
			merchant_id,
			game_code,
			toStartOfHour(settled_at) AS hour,
			toFloat64(sum(bet_amount)) / %d AS total_bet,
			toFloat64(sum(win_amount)) / %d AS total_win,
			count() AS total_rounds,
			if(sum(bet_amount) = 0, 0, toFloat64(sum(win_amount)) / toFloat64(sum(bet_amount))) AS actual_rtp
		FROM fastgame.game_round_settled
		WHERE settled_at >= ?
	`, scale, scale)
	args := []any{since}
	if merchantID > 0 {
		query += " AND merchant_id = ?"
		args = append(args, merchantID)
	}
	if gameCode != "" {
		query += " AND game_code = ?"
		args = append(args, gameCode)
	}
	query += " GROUP BY merchant_id, game_code, hour ORDER BY hour DESC LIMIT 500"

	rows, err := w.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("rtp report: %w", err)
	}
	defer rows.Close()

	var result []RtpReportRow
	for rows.Next() {
		var row RtpReportRow
		if err := rows.Scan(
			&row.MerchantID,
			&row.GameCode,
			&row.Hour,
			&row.TotalBet,
			&row.TotalWin,
			&row.TotalRounds,
			&row.ActualRtp,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
