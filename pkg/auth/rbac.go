package auth

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// adminOnlyPrefixes 只有超管(Admin)才能访问的敏感管理资源
var adminOnlyPrefixes = []string{
	"/api/v1/users",
	"/api/v1/roles",
	"/users",
	"/roles",
}

// adminOnlyMutations 仅超管(Admin)可发起的写操作(POST/PUT/DELETE)
// 包含新建商户、密钥轮换、游戏下发、人工资金调账等高危操作
var adminOnlyMutations = []struct {
	method string
	path   string
}{
	{http.MethodPost, "/merchants"},
	{http.MethodPost, "/rotate-key"},
	{http.MethodPost, "/platform/games"},
	{http.MethodPost, "/ledger/adjustments"},
}

// CanAccess 校验角色是否具备对指定 HTTP 方法和路径的访问权限
func CanAccess(role, method, rawPath string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	method = strings.ToUpper(strings.TrimSpace(method))
	path := cleanPath(rawPath)

	// 1. 公开或 TOTP 双因素认证中继接口放行
	if strings.Contains(path, "/totp/") || strings.HasSuffix(path, "/totp") {
		return true
	}

	// 2. 超管拥有全量操作权限
	if role == RoleAdmin {
		return true
	}

	// 3. 用户与角色管理：仅超管
	for _, prefix := range adminOnlyPrefixes {
		if strings.HasPrefix(path, prefix) || strings.Contains(path, prefix+"/") {
			return false
		}
	}

	// 4. 根据角色分级判断
	switch role {
	case RoleOperator:
		// 读请求全量放行
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			return true
		}
		// 写请求：检查是否命中超管专有高危操作
		return operatorCanMutate(method, path)

	case RoleViewer:
		// 观察员角色仅允许读操作
		return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions

	default:
		return false
	}
}

// operatorCanMutate 判定操作员是否有权执行当前写操作
func operatorCanMutate(method, path string) bool {
	// 仅允许标准写动词
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete && method != http.MethodPatch {
		return false
	}

	// 拦截属于超管专有的高危业务写操作
	for _, m := range adminOnlyMutations {
		if method == m.method {
			// 精准后缀或路径包含匹配
			if strings.HasSuffix(path, m.path) || strings.Contains(path, m.path+"/") || strings.Contains(path, m.path) {
				return false
			}
		}
	}

	return true
}

// cleanPath 剥离 Query 参数、URL 编码及尾部多余斜杠
func cleanPath(p string) string {
	// 剥离可能存在的 Query 字符串
	if idx := strings.IndexByte(p, '?'); idx != -1 {
		p = p[:idx]
	}
	// URL 解码
	if unescaped, err := url.PathUnescape(p); err == nil {
		p = unescaped
	}
	// 清理多余的尾部斜杠
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	return p
}
