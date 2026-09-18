// Package outbox 实现事务性发件箱（transactional outbox）模式。
//
// 解决的问题：原实现由业务代码裸 `go func()` 直接投递 Kafka，且 producer 未设
// RequiredAcks（kafka-go 默认 RequireNone），投递失败无人知晓；消费端又是
// 先 commit offset 再落库，落库失败整批丢失。结果是"扣了钱但事件丢了"、
// 账目与 ClickHouse 对不上。
//
// 本包的做法（借鉴并重写 platform-api 的 internal/app/outbox，适配 go-zero + sqlx）：
//   - 业务与"事件已登记"在**同一个事务**里提交（Store.InsertInTx），
//     事务提交成功即意味着事件不会丢；
//   - Dispatcher 从表里 Claim（FOR UPDATE SKIP LOCKED）→ 投递 → Settle，
//     投递失败按退避重试，投递保证为 at-least-once；
//   - 消费端用 processed_events 做 DB 兜底幂等，且占位行必须与业务写同事务
//     （见 Idempotency.AcquireInTx），避免"幽灵占位行"导致事件被错误 ACK。
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 投递状态
const (
	StatusPending  int64 = 0 // 待投递
	StatusInFlight int64 = 1 // 已被某个 dispatcher 领取
	StatusSent     int64 = 2 // 投递成功
	StatusFailed   int64 = 3 // 超过最大重试，需人工处理
)

// 投递模式（当前仅使用 Kafka，保留位以便后续扩展）
const (
	DeliveryKafka  int64 = 1
	DeliveryPubSub int64 = 2
	DeliveryBoth   int64 = 3
)

// EventTopic 事件 topic 名
type EventTopic string

// Executor 抽象"能执行 SQL 的东西"。
// sqlx.SqlConn 与 sqlx.Session 的方法签名完全一致，因此都满足本接口：
// 同一份仓储代码既可独立执行，也可并入调用方已开启的事务（传 sqlx.Session）。
type Executor interface {
	ExecCtx(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowCtx(ctx context.Context, v any, query string, args ...any) error
	QueryRowsCtx(ctx context.Context, v any, query string, args ...any) error
}

// Transactional 表示该连接能在自身内部开启事务（sqlx.SqlConn 满足）。
// 需要"业务写 + 事件登记"原子提交时用它；仅登记事件时用 Executor 即可。
type Transactional interface {
	Executor
	TransactCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error
}

// Record 一行发件箱记录
type Record struct {
	ID           string
	Topic        string
	MerchantID   uint64 // 0 表示无商户维度
	PartitionKey string
	Payload      json.RawMessage
	DeliveryMode int64
	RetryCount   int64 // 已投递失败次数，SettleBatch 据此判断是否超过上限
}

// Envelope 事件信封：投递到 Kafka 的完整消息体。
// TraceID 让消费端继承上游链路，跨进程串联 trace。
type Envelope struct {
	EventID   string          `json:"eventId"`
	Topic     string          `json:"topic"`
	Source    string          `json:"source"`
	Timestamp int64           `json:"timestamp"` // UnixMilli
	TraceID   string          `json:"traceId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEventID 生成事件 ID。
// 用 UUID v7 而非 v4：v7 高位是毫秒时间戳、单调递增，字典序即时间序，
// 主键索引写入是顺序追加而非随机插入（与 ULID 的选型理由一致）。
func NewEventID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("outbox: generate event id: %w", err)
	}
	return id.String(), nil
}

// NewEnvelope 组装信封，并自行生成 eventId。
//
// 注意：它无法把 eventId 写进 payload（payload 在此之前已定型）。若业务事件体
// 自身也带 eventId 字段并要求与信封一致，请改用 NewEnvelopeWithID：
// 先取 ID → 写进事件体 → 再序列化，否则内层 eventId 会是空值。
func NewEnvelope(topic EventTopic, source, traceID string, payload any, now time.Time) ([]byte, error) {
	id, err := NewEventID()
	if err != nil {
		return nil, err
	}
	return NewEnvelopeWithID(topic, source, traceID, id, payload, now)
}

// NewEnvelopeWithID 用调用方给定的 eventID 组装信封。
// 调用方应先把同一个 ID 写进 payload 结构体，保证内外 eventId 一致，
// 便于消费端对账与排查。
func NewEnvelopeWithID(topic EventTopic, source, traceID, eventID string, payload any, now time.Time) ([]byte, error) {
	if eventID == "" {
		return nil, fmt.Errorf("outbox: empty event id")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outbox: marshal payload: %w", err)
	}
	return json.Marshal(Envelope{
		EventID:   eventID,
		Topic:     string(topic),
		Source:    source,
		Timestamp: now.UnixMilli(),
		TraceID:   traceID,
		Payload:   body,
	})
}

// EnvelopeEventID 从已序列化的信封里取出 eventId（供消费端做幂等键）。
func EnvelopeEventID(raw []byte) (string, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", fmt.Errorf("outbox: unmarshal envelope: %w", err)
	}
	if env.EventID == "" {
		return "", fmt.Errorf("outbox: envelope missing eventId")
	}
	return env.EventID, nil
}

// rawToDB MySQL 的 JSON 列不接受 []byte 直接写入（驱动会当二进制串），
// 因此入库前统一转成 string。
func rawToDB(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
