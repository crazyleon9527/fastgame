package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AuditLog 对应表 audit_logs：后台操作审计日志
type AuditLog struct {
	Id           uint64         `db:"id"`            // 主键
	AdminUserId  sql.NullInt64  `db:"admin_user_id"` // 后台用户 ID（admin_users.id）
	Username     string         `db:"username"`      // 用户名
	RoleName     string         `db:"role_name"`     // 角色名
	Action       string         `db:"action"`        // 操作动作，如 create_merchant/rotate_key/confirm_settlement
	ResourceType string         `db:"resource_type"` // 资源类型：merchant/game/config/user/settlement
	ResourceId   string         `db:"resource_id"`   // 资源 ID
	HttpMethod   sql.NullString `db:"http_method"`   // HTTP 方法
	RequestPath  sql.NullString `db:"request_path"`  // 请求路径
	Detail       sql.NullString `db:"detail"`        // 请求快照或变更差异
	ClientIp     sql.NullString `db:"client_ip"`     // 客户端 IP
	StatusCode   sql.NullInt64  `db:"status_code"`   // HTTP 状态码
	CreatedAt    time.Time      `db:"created_at"`    // 创建时间（UTC）
}

type AuditLogsModel interface {
	ListPage(ctx context.Context, action, username string, page, pageSize int) ([]*AuditLog, int64, error)
}

type defaultAuditLogsModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewAuditLogsModel(conn sqlx.SqlConn) AuditLogsModel {
	return &defaultAuditLogsModel{conn: conn, table: "`audit_logs`"}
}

func (m *defaultAuditLogsModel) ListPage(ctx context.Context, action, username string, page, pageSize int) ([]*AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	where := "1=1"
	args := []any{}
	if action != "" {
		where += " AND action = ?"
		args = append(args, action)
	}
	if username != "" {
		where += " AND username = ?"
		args = append(args, username)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`select id, admin_user_id, username, role_name, action, resource_type, resource_id,
http_method, request_path, detail, client_ip, status_code, created_at
from %s where %s order by id desc limit ? offset ?`, m.table, where)
	listArgs := append(append([]any{}, args...), pageSize, offset)
	var list []*AuditLog
	if err := m.conn.QueryRowsCtx(ctx, &list, query, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
