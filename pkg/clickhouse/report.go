package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fastgame/pkg/money"
)

type RtpReportRow struct {
	MerchantID  uint64    `json:"merchantId"`
	GameCode    string    `json:"gameCode"`
	Hour        time.Time `json:"hour"`
	TotalBet    float64   `json:"totalBet"`
	TotalWin    float64   `json:"totalWin"`
	TotalRounds uint64    `json:"totalRounds"`
	ActualRtp   float64   `json:"actualRtp"`
}

func (w *Writer) QueryRtpReport(ctx context.Context, merchantID uint64, gameCode string, hours int) ([]RtpReportRow, error) {
	if w == nil || w.conn == nil {
		return nil, nil
	}
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`
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
	`, money.Scale, money.Scale))

	args := []any{since}
	if merchantID > 0 {
		sb.WriteString(" AND merchant_id = ?")
		args = append(args, merchantID)
	}
	if gameCode != "" {
		sb.WriteString(" AND game_code = ?")
		args = append(args, gameCode)
	}
	sb.WriteString(" ORDER BY hour DESC LIMIT 500")

	rows, err := w.conn.Query(ctx, sb.String(), args...)
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
	MerchantID  uint64    `json:"merchantId"`
	SettleDate  time.Time `json:"settleDate"`
	TotalBet    int64     `json:"totalBet"` // Minor units (分)
	TotalWin    int64     `json:"totalWin"` // Minor units (分)
	TotalRounds uint64    `json:"totalRounds"`
}

func (w *Writer) QueryDailySettlement(ctx context.Context, merchantID uint64, days int) ([]DailySettlementRow, error) {
	if w == nil || w.conn == nil {
		return nil, nil
	}
	if days <= 0 {
		days = 7
	}
	// 自然日 00:00:00 UTC 对齐
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, time.UTC)

	var sb strings.Builder
	sb.WriteString(`
		SELECT
			merchant_id,
			toDate(settled_at) AS settle_date,
			sum(bet_amount) AS total_bet,
			sum(win_amount) AS total_win,
			count() AS total_rounds
		FROM fastgame.game_round_settled
		WHERE settled_at >= ?
	`)

	args := []any{since}
	if merchantID > 0 {
		sb.WriteString(" AND merchant_id = ?")
		args = append(args, merchantID)
	}
	sb.WriteString(" GROUP BY merchant_id, settle_date ORDER BY settle_date DESC, merchant_id ASC LIMIT 500")

	rows, err := w.conn.Query(ctx, sb.String(), args...)
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

// QueryRtpReportFromRaw 当物化视图为空或重建时的明细兜底查询
func (w *Writer) QueryRtpReportFromRaw(ctx context.Context, merchantID uint64, gameCode string, hours int) ([]RtpReportRow, error) {
	if w == nil || w.conn == nil {
		return nil, nil
	}
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`
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
	`, money.Scale, money.Scale))

	args := []any{since}
	if merchantID > 0 {
		sb.WriteString(" AND merchant_id = ?")
		args = append(args, merchantID)
	}
	if gameCode != "" {
		sb.WriteString(" AND game_code = ?")
		args = append(args, gameCode)
	}
	sb.WriteString(" GROUP BY merchant_id, game_code, hour ORDER BY hour DESC LIMIT 500")

	rows, err := w.conn.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("rtp report raw: %w", err)
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
