package middleware

import (
	"net/http"

	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/logx"
)

func TraceMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			traceID := trace.IDFromRequest(r)
			ctx := trace.WithID(r.Context(), traceID)
			w.Header().Set(trace.HeaderTraceID, traceID)
			logx.WithContext(ctx).Infof("[trace=%s] %s %s", traceID, r.Method, r.URL.Path)
			next(w, r.WithContext(ctx))
		}
	}
}
