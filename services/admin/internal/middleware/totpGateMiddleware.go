package middleware

import (
	"net/http"
	"strings"
)

func jwtClaimBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case string:
		return t == "true" || t == "1"
	default:
		return false
	}
}

// totpGateEnabled 上线前改为 true，未完成 2FA 绑定的账号将只能访问 /totp/* 接口
const totpGateEnabled = false

// TotpGateMiddleware 未完成 2FA 绑定的账号仅允许访问 TOTP 相关接口
func TotpGateMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !totpGateEnabled {
				next(w, r)
				return
			}
			path := r.URL.Path
			if strings.Contains(path, "/totp/") {
				next(w, r)
				return
			}
			if jwtClaimBool(r.Context().Value("totpPending")) {
				http.Error(w, "totp setup required before accessing admin APIs", http.StatusForbidden)
				return
			}
			next(w, r)
		}
	}
}
