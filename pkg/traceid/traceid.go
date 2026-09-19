// Package traceid 只放链路 ID 的最小原语。
//
// 单独成包是为了打断依赖环：pkg/trace 的埋点写入用 pkg/async 派发，
// pkg/async 又要把 trace_id 继承到任务 context 里。两者只共享这里的
// 三行逻辑，不共享 ClickHouse 写入器，因此抽成叶子包最省事。
package traceid

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const HeaderTraceID = "X-Trace-Id"
const HeaderRequestID = "X-Request-Id"

type ctxKey struct{}

// IDFromRequest 从 HTTP Header 读取链路 ID，没有就生成一个。
func IDFromRequest(r *http.Request) string {
	if r == nil {
		return uuid.NewString()
	}
	// 优先提取统一的 X-Trace-Id[cite: 30]
	if id := strings.TrimSpace(r.Header.Get(HeaderTraceID)); id != "" {
		return id
	}
	// 兼容可能由网关注入的 X-Request-Id
	if id := strings.TrimSpace(r.Header.Get(HeaderRequestID)); id != "" {
		return id
	}
	return uuid.NewString()
}

// WithID 把链路 ID 放进 context；传空字符串表示新生成一个。
func WithID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID == "" {
		traceID = uuid.NewString()
	}
	return context.WithValue(ctx, ctxKey{}, traceID)
}

// ID 取出 context 里的链路 ID，没有则返回空串。
func ID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok && v != "" {
		return v
	}
	return ""
}
