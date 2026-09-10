package trace

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const HeaderTraceID = "X-Trace-Id"

type ctxKey struct{}

// IDFromRequest reads or generates a trace id from HTTP headers.
func IDFromRequest(r *http.Request) string {
	if id := strings.TrimSpace(r.Header.Get(HeaderTraceID)); id != "" {
		return id
	}
	return uuid.NewString()
}

func WithID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		traceID = uuid.NewString()
	}
	return context.WithValue(ctx, ctxKey{}, traceID)
}

func ID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok && v != "" {
		return v
	}
	return ""
}
