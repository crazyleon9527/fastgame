package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Deliverer interface {
	Deliver(ctx context.Context, records []*Record) []error
}

type Dispatcher struct {
	conn      Transactional
	store     *Store
	deliverer Deliverer
	backoff   Backoff
	owner     string
	batchSize int
}

type DispatcherConfig struct {
	Owner     string
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

func (d *Dispatcher) DispatchOnce(ctx context.Context) (int, error) {
	var claimed []*Record

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

	results := d.deliverer.Deliver(ctx, claimed)

	failed, err := d.store.SettleBatch(ctx, d.owner, claimed, results, d.backoff)
	if err != nil {
		return 0, err
	}
	if len(failed) > 0 {
		logx.Errorf("[OUTBOX] %d event(s) exceeded max retries and were marked FAILED: %v (需要人工处理：SELECT * FROM event_outbox WHERE status=3)", len(failed), failed)
	}
	return len(claimed), nil
}

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

type Maintainer struct {
	store       *Store
	idem        *Idempotency
	redis       *redis.Client
	staleAfter  time.Duration
	sentRetain  time.Duration
	idemRetain  time.Duration
	gcBatchSize int
}

type MaintainerConfig struct {
	Redis       *redis.Client // 可选：提供 Redis 客户端实现多副本分布式排他锁
	StaleAfter  time.Duration
	SentRetain  time.Duration
	IdemRetain  time.Duration
	GCBatchSize int
}

func NewMaintainer(conn Executor, cfg MaintainerConfig) *Maintainer {
	m := &Maintainer{
		store:       NewStore(conn),
		idem:        NewIdempotency(conn),
		redis:       cfg.Redis,
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

func (m *Maintainer) MaintainOnce(ctx context.Context) error {
	// 如果配置了 Redis，抢占分布式排他锁（有效期 50 秒），避免多副本齐步大表全表扫描与死锁
	if m.redis != nil {
		locked, err := m.redis.SetNX(ctx, "lock:outbox:maintainer", "1", 50*time.Second).Result()
		if err != nil {
			return err
		}
		if !locked {
			// 未拿到锁说明其他 Pod 正在执行维护，直接跳过即可
			return nil
		}
		defer m.redis.Del(ctx, "lock:outbox:maintainer")
	}

	if n, err := m.store.ReapStuck(ctx, m.staleAfter); err != nil {
		return err
	} else if n > 0 {
		logx.Infof("[OUTBOX] reaped %d stuck IN_FLIGHT event(s)", n)
	}

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

type MessageWriter interface {
	PublishRaw(ctx context.Context, topic, key string, body []byte) error
}

type KafkaDeliverer struct {
	writer MessageWriter
}

func NewKafkaDeliverer(w MessageWriter) *KafkaDeliverer {
	return &KafkaDeliverer{writer: w}
}

func (k *KafkaDeliverer) Deliver(ctx context.Context, records []*Record) []error {
	errs := make([]error, len(records))
	for i, rec := range records {
		if rec.Topic == "" {
			errs[i] = fmt.Errorf("outbox: record %s has empty topic", rec.ID)
			continue
		}
		key := rec.PartitionKey
		if key == "" {
			key = rec.ID
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
