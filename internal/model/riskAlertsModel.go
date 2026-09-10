package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type RiskAlert struct {
	Id           uint64    `db:"id"`
	AlertType    string    `db:"alert_type"`
	ScopeType    string    `db:"scope_type"`
	ScopeValue   string    `db:"scope_value"`
	MerchantCode string    `db:"merchant_code"`
	GameCode     string    `db:"game_code"`
	RtpPPM       int64     `db:"rtp_ppm"`
	TotalBet     int64     `db:"total_bet"`
	TotalWin     int64     `db:"total_win"`
	SampleSize   int64     `db:"sample_size"`
	ActionTaken  string    `db:"action_taken"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
}

type RiskAlertsModel interface {
	Insert(ctx context.Context, data *RiskAlert) error
	ListOpen(ctx context.Context, limit int) ([]*RiskAlert, error)
}

type defaultRiskAlertsModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewRiskAlertsModel(conn sqlx.SqlConn) RiskAlertsModel {
	return &defaultRiskAlertsModel{conn: conn, table: "`risk_alerts`"}
}

func (m *defaultRiskAlertsModel) Insert(ctx context.Context, data *RiskAlert) error {
	query := fmt.Sprintf(`
insert into %s (
  alert_type, scope_type, scope_value, merchant_code, game_code,
  rtp_ppm, total_bet, total_win, sample_size, action_taken, status
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, m.table)
	status := data.Status
	if status == "" {
		status = "open"
	}
	_, err := m.conn.ExecCtx(ctx, query,
		data.AlertType, data.ScopeType, data.ScopeValue, data.MerchantCode, data.GameCode,
		data.RtpPPM, data.TotalBet, data.TotalWin, data.SampleSize, data.ActionTaken, status,
	)
	return err
}

func (m *defaultRiskAlertsModel) ListOpen(ctx context.Context, limit int) ([]*RiskAlert, error) {
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf("select * from %s where status = 'open' order by created_at desc limit ?", m.table)
	var list []*RiskAlert
	if err := m.conn.QueryRowsCtx(ctx, &list, query, limit); err != nil {
		return nil, err
	}
	return list, nil
}
