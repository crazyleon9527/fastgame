package trace

import "github.com/prometheus/client_golang/prometheus"

var (
	// spanDroppedCounter 记录由于本地缓冲区满或服务退出导致的丢弃总量
	spanDroppedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "fastgame",
			Subsystem: "trace",
			Name:      "spans_dropped_total",
			Help:      "Total number of trace spans dropped due to buffer overflow or shutdown",
		},
	)

	// spanRecordedCounter 记录成功进入本地缓冲队列的 Span 总量
	spanRecordedCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "fastgame",
			Subsystem: "trace",
			Name:      "spans_enqueued_total",
			Help:      "Total number of trace spans successfully enqueued to buffer",
		},
	)

	// batchFlushCounter 记录批量刷入 ClickHouse 的次数（按 status 区分 success / failed）
	batchFlushCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "fastgame",
			Subsystem: "trace",
			Name:      "batch_flush_total",
			Help:      "Total number of batch flush attempts to ClickHouse",
		},
		[]string{"status"},
	)

	// batchFlushDuration 记录每次批量刷入 ClickHouse 的耗时直方图
	batchFlushDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "fastgame",
			Subsystem: "trace",
			Name:      "batch_flush_duration_seconds",
			Help:      "Duration of ClickHouse batch insert operations in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
	)

	// queueLengthGauge 实时反映本地队列当前排队堆积长度
	queueLengthGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "fastgame",
			Subsystem: "trace",
			Name:      "queue_length",
			Help:      "Current number of spans pending in local memory queue",
		},
	)
)

func init() {
	prometheus.MustRegister(
		spanDroppedCounter,
		spanRecordedCounter,
		batchFlushCounter,
		batchFlushDuration,
		queueLengthGauge,
	)
}
