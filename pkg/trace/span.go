package trace

import (
	"context"
	"time"
)

// Span is a single I/O step persisted to ClickHouse for troubleshooting.
type Span struct {
	TraceID    string
	SpanID     string
	Service    string
	Operation  string
	RoundID    string
	Status     string
	Detail     string
	DurationMs uint32
	OccurredAt time.Time
}

type Recorder func(ctx context.Context, span Span)
