package middleware

import (
	"context"
	"net/http"

	"fastgame/pkg/auth"
)

func roleFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value("roleName").(string); ok && v != "" {
		return v
	}
	v := ctx.Value("roleName")
	switch name := v.(type) {
	case string:
		return name
	default:
		return ""
	}
}

func denyPermission(w http.ResponseWriter) {
	http.Error(w, "permission denied", http.StatusForbidden)
}

func checkRBAC(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	role := roleFromCtx(r.Context())
	if role == "" {
		http.Error(w, "token missing roleName, please login again", http.StatusUnauthorized)
		return false
	}
	if !auth.CanAccess(role, r.Method, r.URL.Path) {
		denyPermission(w)
		return false
	}
	return true
}
