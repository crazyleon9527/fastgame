package trace

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"fastgame/pkg/clickhouse"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultBatchSize    = 1000            // 满 1000 条触发批量刷盘
	defaultFlushTimeout = 1 * time.Second // 最长等待 1 秒刷盘一次
	defaultChanBuffer   = 10000           // 本地内存缓冲深度
)

type CHRecorder struct {
	writer *clickhouse.Writer
	queue  chan clickhouse.TraceSpanRow

	dropped atomic.Int64
	closed  atomic.Bool
	done    chan struct{}
	wg      sync.WaitGroup
}

func NewCHRecorder(w *clickhouse.Writer) *CHRecorder {
	r := &CHRecorder{
		writer: w,
		queue:  make(chan clickhouse.TraceSpanRow, defaultChanBuffer),
		done:   make(chan struct{}),
	}

	r.wg.Add(1)
	go r.batchWorker()

	return r
}

func (r *CHRecorder) batchWorker() {
	defer r.wg.Done()

	batch := make([]clickhouse.TraceSpanRow, 0, defaultBatchSize)
	ticker := time.NewTicker(defaultFlushTimeout)
	defer ticker.Stop()

	// 定时采样队列堆积水位的 Ticker
	queueMetricTicker := time.NewTicker(200 * time.Millisecond)
	defer queueMetricTicker.Stop()

	flush := func() {
		if len(batch) == 0 || r.writer == nil {
			return
		}

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := r.writer.BatchInsertTraceSpans(ctx, batch)
		cancel()

		duration := time.Since(start).Seconds()
		batchFlushDuration.Observe(duration)

		if err != nil {
			batchFlushCounter.WithLabelValues("failed").Inc()
			logx.Errorw("trace_batch_flush_failed",
				logx.Field("count", len(batch)),
				logx.Field("duration_sec", duration),
				logx.Field("err", err),
			)
		} else {
			batchFlushCounter.WithLabelValues("success").Inc()
		}

		batch = make([]clickhouse.TraceSpanRow, 0, defaultBatchSize)
	}

	for {
		select {
		case <-r.done:
			for {
				select {
				case row := <-r.queue:
					batch = append(batch, row)
					if len(batch) >= defaultBatchSize {
						flush()
					}
				default:
					flush()
					queueLengthGauge.Set(0)
					return
				}
			}

		case row := <-r.queue:
			batch = append(batch, row)
			if len(batch) >= defaultBatchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case <-queueMetricTicker.C:
			queueLengthGauge.Set(float64(len(r.queue)))
		}
	}
}

// Close 优雅停机并清空队列
func (r *CHRecorder) Close(ctx context.Context) error {
	if r == nil || r.closed.Swap(true) {
		return nil
	}
	close(r.done)

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *CHRecorder) Record(ctx context.Context, span Span) {
	if r == nil || r.writer == nil || r.closed.Load() {
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

	// 截断超大报文，避免击穿存储
	detail := span.Detail
	if len(detail) > 2048 {
		detail = detail[:2048] + "...(truncated)"
	}

	row := clickhouse.TraceSpanRow{
		TraceID:    traceID,
		SpanID:     spanID,
		Service:    span.Service,
		Operation:  span.Operation,
		RoundID:    span.RoundID,
		Status:     span.Status,
		Detail:     detail,
		DurationMs: span.DurationMs,
		OccurredAt: occurredAt,
	}

	select {
	case r.queue <- row:
		spanRecordedCounter.Inc()
	default:
		r.dropped.Add(1)
		spanDroppedCounter.Inc()
	}
}

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
