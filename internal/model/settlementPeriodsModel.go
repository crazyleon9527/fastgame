package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// SettlementPeriod 对应表 settlement_periods：商户结算周期账单
type SettlementPeriod struct {
	Id              uint64         `db:"id"`                // 主键
	MerchantId      uint64         `db:"merchant_id"`       // 商户 ID（merchants.id）
	PeriodType      string         `db:"period_type"`       // 周期类型：daily/weekly/monthly
	PeriodStart     time.Time      `db:"period_start"`      // 结算周期开始（UTC）
	PeriodEnd       time.Time      `db:"period_end"`        // 结算周期结束（UTC）
	CurrencyCode    string         `db:"currency_code"`     // 币种编码（currencies.code）
	TotalBetMinor   int64          `db:"total_bet_minor"`   // 总下注额，minor units
	TotalWinMinor   int64          `db:"total_win_minor"`   // 总派彩额，minor units
	GgrMinor        int64          `db:"ggr_minor"`         // GGR = 总下注 − 总派彩
	CommissionMinor int64          `db:"commission_minor"`  // 平台从 GGR 中抽取的分成
	NetPayableMinor int64          `db:"net_payable_minor"` // 应付净额（平台应收）
	TotalRounds     uint64         `db:"total_rounds"`      // 总局数
	Status          string         `db:"status"`            // 状态：draft/pending_review/confirmed/invoiced/paid
	ConfirmedBy     sql.NullInt64  `db:"confirmed_by"`      // 确认人（admin_users.id）
	ConfirmedAt     sql.NullTime   `db:"confirmed_at"`      // 确认时间（UTC）
	Notes           sql.NullString `db:"notes"`             // 备注
	CreatedAt       time.Time      `db:"created_at"`        // 创建时间（UTC）
	UpdatedAt       time.Time      `db:"updated_at"`        // 更新时间（UTC）
}

type SettlementPeriodsModel interface {
	ListPage(ctx context.Context, merchantID uint64, periodType string, page, pageSize int) ([]*SettlementPeriod, int64, error)
	FindOne(ctx context.Context, id uint64) (*SettlementPeriod, error)
	GenerateFromDaily(ctx context.Context, merchantID uint64, periodType string, start, end time.Time, currencyCode string) (int64, error)
	SetPeriodOnDaily(ctx context.Context, merchantID uint64, periodID uint64, start, end time.Time) error
}

type defaultSettlementPeriodsModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewSettlementPeriodsModel(conn sqlx.SqlConn) SettlementPeriodsModel {
	return &defaultSettlementPeriodsModel{
		conn:  conn,
		table: "`settlement_periods`",
	}
}

func (m *defaultSettlementPeriodsModel) ListPage(ctx context.Context, merchantID uint64, periodType string, page, pageSize int) ([]*SettlementPeriod, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "1=1"
	args := []any{}
	if merchantID > 0 {
		where += " AND merchant_id = ?"
		args = append(args, merchantID)
	}
	if periodType != "" {
		where += " AND period_type = ?"
		args = append(args, periodType)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		"select id, merchant_id, period_type, period_start, period_end, currency_code, total_bet_minor, total_win_minor, ggr_minor, commission_minor, net_payable_minor, total_rounds, status, confirmed_by, confirmed_at, notes, created_at, updated_at from %s where %s order by period_end desc, merchant_id asc limit ? offset ?",
		m.table, where,
	)
	listArgs := append(append([]any{}, args...), pageSize, offset)
	var list []*SettlementPeriod
	if err := m.conn.QueryRowsCtx(ctx, &list, query, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultSettlementPeriodsModel) FindOne(ctx context.Context, id uint64) (*SettlementPeriod, error) {
	query := fmt.Sprintf(
		"select id, merchant_id, period_type, period_start, period_end, currency_code, total_bet_minor, total_win_minor, ggr_minor, commission_minor, net_payable_minor, total_rounds, status, confirmed_by, confirmed_at, notes, created_at, updated_at from %s where id = ? limit 1",
		m.table,
	)
	var row SettlementPeriod
	err := m.conn.QueryRowCtx(ctx, &row, query, id)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// GenerateFromDaily rolls up confirmed daily_settlements rows within [start, end]
// into a single settlement_period row (upsert by merchant+type+range). Returns the
// period id (existing or newly created).
func (m *defaultSettlementPeriodsModel) GenerateFromDaily(ctx context.Context, merchantID uint64, periodType string, start, end time.Time, currencyCode string) (int64, error) {
	query := fmt.Sprintf(`
		insert into %s
			(merchant_id, period_type, period_start, period_end, currency_code,
			 total_bet_minor, total_win_minor, ggr_minor, total_rounds, status)
		select merchant_id, ?, ?, ?, ?,
		       sum(total_bet), sum(total_win), sum(total_bet) - sum(total_win), sum(total_rounds), 'draft'
		from daily_settlements
		where merchant_id = ? and status = 1 and settle_date between ? and ?
		group by merchant_id
		on duplicate key update
			total_bet_minor = values(total_bet_minor),
			total_win_minor = values(total_win_minor),
			ggr_minor       = values(ggr_minor),
			total_rounds    = values(total_rounds),
			updated_at      = current_timestamp(3)
	`, m.table)
	res, err := m.conn.ExecCtx(ctx, query,
		periodType, start.Format("2006-01-02"), end.Format("2006-01-02"), currencyCode,
		merchantID, start.Format("2006-01-02"), end.Format("2006-01-02"),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id > 0 {
		return id, nil
	}
	// Upsert on an existing unique key: fetch the existing period id.
	var pid int64
	lookup := fmt.Sprintf(
		"select id from %s where merchant_id = ? and period_type = ? and period_start = ? and period_end = ? limit 1",
		m.table,
	)
	if err := m.conn.QueryRowCtx(ctx, &pid, lookup,
		merchantID, periodType, start.Format("2006-01-02"), end.Format("2006-01-02"),
	); err != nil {
		return 0, err
	}
	return pid, nil
}

// SetPeriodOnDaily backfills daily_settlements.settlement_period_id for confirmed
// rows inside the period range (idempotent: does not overwrite an existing link).
func (m *defaultSettlementPeriodsModel) SetPeriodOnDaily(ctx context.Context, merchantID uint64, periodID uint64, start, end time.Time) error {
	query := `
		update daily_settlements
		set settlement_period_id = ?
		where merchant_id = ? and status = 1
		  and settle_date between ? and ?
		  and settlement_period_id is null
	`
	_, err := m.conn.ExecCtx(ctx, query, periodID, merchantID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	return err
}
