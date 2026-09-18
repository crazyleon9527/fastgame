package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Deliverer 把一批记录投递出去。
//
// 约定：返回的 errors 切片与 records 等长，nil 表示该条投递成功。
// 之所以按"每条一个 error"而不是整体成败，是因为批量投递里可能出现部分成功
// （例如 Kafka 对单条消息返回错误），必须逐条结算，否则整批误标 SENT 会造成
// 事件永久丢失——这是本模式最深的坑。
type Deliverer interface {
	Deliver(ctx context.Context, records []*Record) []error
}

// Dispatcher 三段式派发器：Claim → Deliver → Settle。
type Dispatcher struct {
	conn      Transactional
	store     *Store
	deliverer Deliverer
	backoff   Backoff
	owner     string
	batchSize int
}

// DispatcherConfig 派发器配置
type DispatcherConfig struct {
	Owner     string // 实例标识，形如 "rgs-api:12345"；Settle 时校验归属
	BatchSize int
}

func NewDispatcher(conn Transactional, deliverer Deliverer, cfg DispatcherConfig) *Dispatcher {
	size := cfg.BatchSize
	if size <= 0 {
		size = 50
	}
	return &Dispatcher{
		conn:      conn,
		store:     NewStore(conn),
		deliverer: deliverer,
		backoff:   DefaultBackoff(),
		owner:     cfg.Owner,
		batchSize: size,
	}
}

// DispatchOnce 执行一轮：领取一批 → 投递 → 结算。返回本轮处理条数。
func (d *Dispatcher) DispatchOnce(ctx context.Context) (int, error) {
	var claimed []*Record

	// 1) 领取：短事务 + FOR UPDATE SKIP LOCKED，尽快提交以释放锁
	err := d.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		claimed, err = d.store.ClaimBatch(ctx, session, d.owner, d.batchSize)
		return err
	})
	if err != nil {
		return 0, err
	}
	if len(claimed) == 0 {
		return 0, nil
	}

	// 2) 投递：在事务外执行，避免长时间占着数据库连接与行锁
	results := d.deliverer.Deliver(ctx, claimed)

	// 3) 结算：带 owner+status 双守卫
	failed, err := d.store.SettleBatch(ctx, d.owner, claimed, results, d.backoff)
	if err != nil {
		return 0, err
	}
	if len(failed) > 0 {
		logx.Errorf("[OUTBOX] %d event(s) exceeded max retries and were marked FAILED: %v "+
			"(需要人工处理：SELECT * FROM event_outbox WHERE status=3)", len(failed), failed)
	}
	return len(claimed), nil
}

// Run 常驻循环派发，直到 ctx 取消。
func (d *Dispatcher) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := d.DispatchOnce(ctx); err != nil {
				logx.Errorf("[OUTBOX] dispatch failed: %v", err)
			}
		}
	}
}

// ------------------------------------------------------------------ 维护循环

// Maintainer 负责回收悬挂记录并清理历史数据。
//
// 两者都要抢分布式锁再执行（多实例下只让一个实例跑），
// 否则多副本会重复做整表扫描与删除。
type Maintainer struct {
	store       *Store
	idem        *Idempotency
	staleAfter  time.Duration
	sentRetain  time.Duration
	idemRetain  time.Duration
	gcBatchSize int
}

type MaintainerConfig struct {
	StaleAfter  time.Duration // IN_FLIGHT 悬挂多久后回收
	SentRetain  time.Duration // 已投递记录保留多久
	IdemRetain  time.Duration // 幂等占位保留多久（应 >= 上游最大重投窗口）
	GCBatchSize int
}

func NewMaintainer(conn Executor, cfg MaintainerConfig) *Maintainer {
	m := &Maintainer{
		store:       NewStore(conn),
		idem:        NewIdempotency(conn),
		staleAfter:  cfg.StaleAfter,
		sentRetain:  cfg.SentRetain,
		idemRetain:  cfg.IdemRetain,
		gcBatchSize: cfg.GCBatchSize,
	}
	if m.staleAfter <= 0 {
		m.staleAfter = 2 * time.Minute
	}
	if m.sentRetain <= 0 {
		m.sentRetain = 72 * time.Hour
	}
	if m.idemRetain <= 0 {
		m.idemRetain = 72 * time.Hour
	}
	if m.gcBatchSize <= 0 {
		m.gcBatchSize = 1000
	}
	return m
}

// MaintainOnce 执行一轮回收 + 清理。
func (m *Maintainer) MaintainOnce(ctx context.Context) error {
	if n, err := m.store.ReapStuck(ctx, m.staleAfter); err != nil {
		return err
	} else if n > 0 {
		logx.Infof("[OUTBOX] reaped %d stuck IN_FLIGHT event(s)", n)
	}

	// 循环删除直到删不满一批（避免单轮删除量无限大）
	for {
		n, err := m.store.PurgeSent(ctx, m.sentRetain, m.gcBatchSize)
		if err != nil {
			return err
		}
		if int(n) < m.gcBatchSize {
			break
		}
	}

	for {
		n, err := m.idem.PurgeProcessed(ctx, m.idemRetain, m.gcBatchSize)
		if err != nil {
			return err
		}
		if int(n) < m.gcBatchSize {
			break
		}
	}
	return nil
}

// Run 常驻维护循环。
func (m *Maintainer) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.MaintainOnce(ctx); err != nil {
				logx.Errorf("[OUTBOX] maintain failed: %v", err)
			}
		}
	}
}

// ------------------------------------------------------------------ Kafka 适配

// MessageWriter 是 pkg/kafka.Producer 需要满足的最小接口（便于测试替换）。
type MessageWriter interface {
	PublishRaw(ctx context.Context, topic, key string, body []byte) error
}

// KafkaDeliverer 把 outbox 记录投递到 Kafka。
type KafkaDeliverer struct {
	writer MessageWriter
}

func NewKafkaDeliverer(w MessageWriter) *KafkaDeliverer {
	return &KafkaDeliverer{writer: w}
}

// Deliver 逐条投递并收集每条的错误。
//
// 逐条而非批量：kafka-go 的 WriteMessages 支持批量，但为精确归因（哪条失败）
// 且避免"整批误标 SENT"，这里逐条投递。outbox 的派发频率不高（默认 1s 一轮），
// 单轮最多 batchSize 条，逐条写入的额外开销远小于数据丢失的代价。
func (k *KafkaDeliverer) Deliver(ctx context.Context, records []*Record) []error {
	errs := make([]error, len(records))
	for i, rec := range records {
		if rec.Topic == "" {
			errs[i] = fmt.Errorf("outbox: record %s has empty topic", rec.ID)
			continue
		}
		key := rec.PartitionKey
		if key == "" {
			key = rec.ID // 兜底：保证同一条记录稳定落到同一分区
		}
		if err := k.writer.PublishRaw(ctx, rec.Topic, key, rec.Payload); err != nil {
			errs[i] = err
		}
	}
	return errs
}

var (
	_ Deliverer = (*KafkaDeliverer)(nil)
	_ Executor  = (sqlx.Session)(nil)
	_           = json.Marshal
)
