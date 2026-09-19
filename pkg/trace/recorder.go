package trace

import (
	"context"
	"sync/atomic"
	"time"

	"fastgame/pkg/async"
	"fastgame/pkg/clickhouse"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type CHRecorder struct {
	writer *clickhouse.Writer
	// spans 负责把埋点写入从请求路径上摘下来。
	// 之前是每写一条 span 起一个裸 goroutine：无上限、panic 会打挂进程、
	// 进程退出时在途写入被静默丢弃。现在有并发闸门 + panic 兜底 + 可 Drain。
	spans   *async.Runner
	dropped atomic.Int64
}

func NewCHRecorder(w *clickhouse.Writer) *CHRecorder {
	return &CHRecorder{
		writer: w,
		spans: async.New("trace-recorder",
			async.WithMaxConcurrency(traceWriterConcurrency),
			async.WithDrainTimeout(traceWriterDrainTimeout),
		),
	}
}

const (
	// traceWriterConcurrency 限制同时在途的 span 写入数。
	// 埋点是旁路数据：宁可丢弃也不能把下游 ClickHouse 和本进程内存拖垮。
	traceWriterConcurrency  = 256
	traceWriterDrainTimeout = 3 * time.Second
)

// Close 等待在途埋点写完，供进程优雅退出时调用。
func (r *CHRecorder) Close(ctx context.Context) error {
	if r == nil || r.spans == nil {
		return nil
	}
	return r.spans.Shutdown(ctx)
}

// Stats 暴露埋点丢弃/panic 计数，便于监控"埋点是否在悄悄丢数据"。
func (r *CHRecorder) Stats() async.Stats {
	if r == nil || r.spans == nil {
		return async.Stats{}
	}
	return r.spans.Stats()
}

func (r *CHRecorder) Record(ctx context.Context, span Span) {
	if r == nil || r.writer == nil {
		return
	}
	traceID := span.TraceID
	if traceID == "" {
		traceID = ID(ctx)
	}
	if traceID == "" {
		return
	}
	spanID := span.SpanID
	if spanID == "" {
		spanID = uuid.NewString()
	}
	occurredAt := span.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	row := clickhouse.TraceSpanRow{
		TraceID:    traceID,
		SpanID:     spanID,
		Service:    span.Service,
		Operation:  span.Operation,
		RoundID:    span.RoundID,
		Status:     span.Status,
		Detail:     span.Detail,
		DurationMs: span.DurationMs,
		OccurredAt: occurredAt,
	}
	// 入队失败（埋点积压到上限或进程正在退出）直接放弃这条 span：
	// 埋点是旁路数据，不能反压到资金主链路。
	ok := r.spans.Go(ctx, func(taskCtx context.Context) {
		c, cancel := context.WithTimeout(taskCtx, 3*time.Second)
		defer cancel()
		if err := r.writer.BatchInsertTraceSpans(c, []clickhouse.TraceSpanRow{row}); err != nil {
			// 字段名与 pkg/log 的 KeyErr/KeyRoundID 保持一致（这里不能 import pkg/log，
			// 否则 pkg/log -> pkg/trace -> pkg/log 形成环）。
			logx.WithContext(taskCtx).Errorw("trace_span_insert_failed",
				logx.Field("err", err),
				logx.Field("round_id", row.RoundID),
			)
		}
	})
	if !ok {
		r.dropped.Add(1)
	}
}

// Dropped 返回因积压而被丢弃的埋点数。
func (r *CHRecorder) Dropped() int64 {
	if r == nil {
		return 0
	}
	return r.dropped.Load()
}

func Record(ctx context.Context, rec *CHRecorder, service, operation, roundID, status, detail string, duration time.Duration) {
	if rec == nil {
		return
	}
	rec.Record(ctx, Span{
		TraceID:    ID(ctx),
		Service:    service,
		Operation:  operation,
		RoundID:    roundID,
		Status:     status,
		Detail:     detail,
		DurationMs: uint32(duration.Milliseconds()),
		OccurredAt: time.Now().UTC(),
	})
}
