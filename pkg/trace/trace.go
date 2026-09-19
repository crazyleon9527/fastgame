package trace

import (
	"context"
	"net/http"

	"fastgame/pkg/traceid"
)

// HeaderTraceID 链路 ID 的 HTTP Header 名（实现见 pkg/traceid）。
const HeaderTraceID = traceid.HeaderTraceID

// IDFromRequest 读取或生成链路 ID。
func IDFromRequest(r *http.Request) string { return traceid.IDFromRequest(r) }

// WithID 把链路 ID 写入 context。
func WithID(ctx context.Context, traceID string) context.Context { return traceid.WithID(ctx, traceID) }

// ID 读取 context 里的链路 ID。
func ID(ctx context.Context) string { return traceid.ID(ctx) }
