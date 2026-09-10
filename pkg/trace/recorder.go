package trace

import (
	"context"
	"time"

	"fastgame/pkg/clickhouse"

	"github.com/google/uuid"
)

type CHRecorder struct {
	writer *clickhouse.Writer
}

func NewCHRecorder(w *clickhouse.Writer) *CHRecorder {
	return &CHRecorder{writer: w}
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
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = r.writer.BatchInsertTraceSpans(c, []clickhouse.TraceSpanRow{row})
	}()
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
