package log

import (
	"net"
	"net/http"
	"strings"
	"time"

	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/logx"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware injects trace_id, binds log fields, and emits one access log per request.
func HTTPMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			traceID := trace.IDFromRequest(r)
			ctx := trace.WithID(r.Context(), traceID)
			ctx = WithFields(ctx,
				logx.Field(KeyTraceID, traceID),
				logx.Field(KeyMethod, r.Method),
				logx.Field(KeyPath, r.URL.Path),
				logx.Field(KeyClientIP, clientIP(r)),
			)
			w.Header().Set(trace.HeaderTraceID, traceID)

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next(sw, r.WithContext(ctx))

			C(ctx).Infow("http_request",
				logx.Field(KeyStatus, sw.status),
				logx.Field(KeyDurationMs, time.Since(start).Milliseconds()),
			)
		}
	}
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
