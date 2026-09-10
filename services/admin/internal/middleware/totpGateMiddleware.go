package middleware

import (
	"net/http"
	"strings"
)

// TotpGateMiddleware 未完成 2FA 绑定的账号仅允许访问 TOTP 相关接口
func TotpGateMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.Contains(path, "/totp/") {
				next(w, r)
				return
			}
			if pending, ok := r.Context().Value("totpPending").(bool); ok && pending {
				http.Error(w, "totp setup required before accessing admin APIs", http.StatusForbidden)
				return
			}
			next(w, r)
		}
	}
}
