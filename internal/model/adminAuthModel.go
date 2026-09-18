package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AdminAuthRecord 对应表 admin_users：管理后台用户
type AdminAuthRecord struct {
	Id                 uint64         `db:"id"`                   // 主键
	Username           string         `db:"username"`             // 用户名
	PasswordHash       string         `db:"password_hash"`        // 密码哈希（bcrypt）
	RoleId             uint64         `db:"role_id"`              // 角色 ID（roles.id）
	Status             int64          `db:"status"`               // 状态：1=启用 0=停用
	TotpSecret         sql.NullString `db:"totp_secret"`          // TOTP 密钥（加密存储）
	TotpEnabled        int64          `db:"totp_enabled"`         // 1=已启用二次验证 0=未启用
	TotpRecoveryHashes sql.NullString `db:"totp_recovery_hashes"` // 一次性恢复码的 bcrypt 哈希
}

type AdminAuthModel interface {
	FindByUsername(ctx context.Context, username string) (*AdminAuthRecord, error)
	FindByID(ctx context.Context, id uint64) (*AdminAuthRecord, error)
	UpdateTotp(ctx context.Context, id uint64, secret string, enabled bool) error
	UpdateRecoveryHashes(ctx context.Context, id uint64, hashesJSON string) error
	ConsumeRecoveryHash(ctx context.Context, id uint64, hashesJSON string) error
}

type defaultAdminAuthModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewAdminAuthModel(conn sqlx.SqlConn) AdminAuthModel {
	return &defaultAdminAuthModel{conn: conn, table: "`admin_users`"}
}

func (m *defaultAdminAuthModel) FindByID(ctx context.Context, id uint64) (*AdminAuthRecord, error) {
	query := fmt.Sprintf(`
select id, username, password_hash, role_id, status, totp_secret, totp_enabled, totp_recovery_hashes
from %s where id = ? limit 1
`, m.table)
	var row AdminAuthRecord
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

func (m *defaultAdminAuthModel) UpdateTotp(ctx context.Context, id uint64, secret string, enabled bool) error {
	query := fmt.Sprintf("update %s set totp_secret = ?, totp_enabled = ? where id = ?", m.table)
	enabledInt := int64(0)
	if enabled {
		enabledInt = 1
	}
	_, err := m.conn.ExecCtx(ctx, query, secret, enabledInt, id)
	return err
}

func (m *defaultAdminAuthModel) UpdateRecoveryHashes(ctx context.Context, id uint64, hashesJSON string) error {
	query := fmt.Sprintf("update %s set totp_recovery_hashes = ? where id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, hashesJSON, id)
	return err
}

func (m *defaultAdminAuthModel) ConsumeRecoveryHash(ctx context.Context, id uint64, hashesJSON string) error {
	query := fmt.Sprintf("update %s set totp_recovery_hashes = ? where id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, hashesJSON, id)
	return err
}

func (m *defaultAdminAuthModel) FindByUsername(ctx context.Context, username string) (*AdminAuthRecord, error) {
	query := fmt.Sprintf(`
select id, username, password_hash, role_id, status, totp_secret, totp_enabled, totp_recovery_hashes
from %s where username = ? limit 1
`, m.table)
	var row AdminAuthRecord
	err := m.conn.QueryRowCtx(ctx, &row, query, username)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
