package middleware

import (
	"net/http"

	"fastgame/pkg/httputil"
	"fastgame/pkg/ratelimit"
	"fastgame/pkg/xerr"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func RateLimitMiddleware(gw *ratelimit.Gateway) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if gw != nil && !gw.AllowIP(r.Context(), httputil.ClientIP(r)) {
				httpx.ErrorCtx(r.Context(), w, xerr.ErrRateLimited)
				return
			}
			next(w, r)
		}
	}
}
