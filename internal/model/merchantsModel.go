package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantsModel = (*customMerchantsModel)(nil)

type (
	// MerchantsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantsModel.
	MerchantSecrets struct {
		PrivateKey              sql.NullString
		PrivateKeyPrev          sql.NullString
		PrivateKeyPrevExpiresAt sql.NullTime
	}

	MerchantsModel interface {
		merchantsModel
		withSession(session sqlx.Session) MerchantsModel
		ListPage(ctx context.Context, page, pageSize int) ([]*Merchants, int64, error)
		FindSecretsByMerchantCode(ctx context.Context, merchantCode string) (*MerchantSecrets, error)
		RotatePrivateKey(ctx context.Context, id uint64, newKey string, gracePeriod time.Duration) error
	}

	customMerchantsModel struct {
		*defaultMerchantsModel
	}
)

// NewMerchantsModel returns a model for the database table.
func NewMerchantsModel(conn sqlx.SqlConn) MerchantsModel {
	return &customMerchantsModel{
		defaultMerchantsModel: newMerchantsModel(conn),
	}
}

func (m *customMerchantsModel) withSession(session sqlx.Session) MerchantsModel {
	return NewMerchantsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customMerchantsModel) ListPage(ctx context.Context, page, pageSize int) ([]*Merchants, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s", m.table)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("select %s from %s order by id desc limit ? offset ?", merchantsRows, m.table)
	var list []*Merchants
	if err := m.conn.QueryRowsCtx(ctx, &list, query, pageSize, offset); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customMerchantsModel) FindSecretsByMerchantCode(ctx context.Context, merchantCode string) (*MerchantSecrets, error) {
	query := fmt.Sprintf(
		"select `private_key`, `private_key_prev`, `private_key_prev_expires_at` from %s where `merchant_code` = ? limit 1",
		m.table,
	)
	var resp MerchantSecrets
	err := m.conn.QueryRowCtx(ctx, &resp, query, merchantCode)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMerchantsModel) RotatePrivateKey(ctx context.Context, id uint64, newKey string, gracePeriod time.Duration) error {
	merchant, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	var prevKey sql.NullString
	var prevExpires sql.NullTime
	if merchant.PrivateKey.Valid && merchant.PrivateKey.String != "" {
		prevKey = merchant.PrivateKey
		prevExpires = sql.NullTime{Time: time.Now().UTC().Add(gracePeriod), Valid: true}
	}

	query := fmt.Sprintf(
		"update %s set `private_key` = ?, `private_key_prev` = ?, `private_key_prev_expires_at` = ? where `id` = ?",
		m.table,
	)
	_, err = m.conn.ExecCtx(ctx, query, newKey, prevKey, prevExpires, id)
	return err
}
