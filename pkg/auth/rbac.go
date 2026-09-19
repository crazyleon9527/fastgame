package auth

import (
	"net/http"
	"strings"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// CanAccess checks whether role may call method+path on admin API.
func CanAccess(role, method, path string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	method = strings.ToUpper(method)

	if strings.Contains(path, "/totp/") {
		return true
	}

	// 用户/角色管理 — 仅超管
	if strings.Contains(path, "/users") || strings.Contains(path, "/roles") {
		return role == RoleAdmin
	}

	switch role {
	case RoleAdmin:
		return true
	case RoleOperator:
		if method == http.MethodGet {
			return true
		}
		return operatorCanWrite(path, method)
	case RoleViewer:
		return method == http.MethodGet
	default:
		return false
	}
}

func operatorCanWrite(path, method string) bool {
	if method == http.MethodGet {
		return true
	}
	// 超管专属：新建商户、轮换密钥
	if method == http.MethodPost && strings.HasSuffix(path, "/merchants") {
		return false
	}
	if method == http.MethodPost && strings.Contains(path, "/rotate-key") {
		return false
	}
	if method == http.MethodPost && strings.HasSuffix(path, "/platform/games") {
		return false
	}
	// 人工调账直接改玩家资金（登记一笔钱包侧的人工加减款），
	// 与"新建商户/轮换密钥"同级，只允许超管发起：运营误操作一次就是资金差错。
	if method == http.MethodPost && strings.HasSuffix(path, "/ledger/adjustments") {
		return false
	}
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete
}
