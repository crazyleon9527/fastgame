package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"fastgame/pkg/async"
	"fastgame/pkg/batch"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/kafka"
	applog "fastgame/pkg/log"
	"fastgame/pkg/outbox"
	"fastgame/pkg/rtpwatchdog"
	"fastgame/services/consumer/internal/svc"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// consumerGroup 是 processed_events 里的消费组名（幂等命名空间）。
// 换名字会让历史幂等记录失效、可能重复处理，因此不要随意改。
const consumerGroup = "fastgame-settle-consumer"

// envelopeTopic 兼容入口：老实现往 topic 里投递裸 RoundSettledEvent，
// 新实现投递 outbox 信封。两种都要能解析，否则滚动升级期间会丢事件。
type envelopeItem struct {
	EventID   string          `json:"eventId"`
	Topic     string          `json:"topic"`
	Source    string          `json:"source"`
	Timestamp int64           `json:"timestamp"`
	TraceID   string          `json:"traceId"`
	Payload   json.RawMessage `json:"payload"`
}

// batchItem 把待写入行与它的原始 Kafka 消息绑在一起。
// 只有这样，flush 成功后才能精确提交这一批的 offset——
// 原实现是"读到消息就提交 offset、随后才批量落库"，
// 落库失败时 offset 已提交，数据永久丢失（at-most-once）。
type batchItem struct {
	row clickhouse.RoundSettledRow
	msg kafkago.Message
}

type Worker struct {
	svcCtx *svc.ServiceContext
	reader *kafkago.Reader
	batch  *batch.Batcher[batchItem]
	// tasks 管理批量刷盘循环：裸 goroutine 里的 panic 会打挂消费进程
	// （Kafka 侧看不到任何错误，只会表现为消费停滞）。
	tasks *async.Runner
}

func NewWorker(svcCtx *svc.ServiceContext, maxSize int, interval time.Duration) *Worker {
	w := &Worker{svcCtx: svcCtx, tasks: async.New("settle-consumer-batch")}
	w.batch = batch.NewBatcher(maxSize, interval, w.flush)
	w.reader = kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  svcCtx.Config.Kafka.Brokers,
		GroupID:  svcCtx.Config.Kafka.GroupID,
		Topic:    svcCtx.Config.Kafka.Topic,
		MinBytes: 1,
		MaxBytes: 10e6,
		// CommitInterval = 0 → 关闭自动提交，改由 flush 成功后手动提交。
		// 这是 at-least-once 的关键：宁可重复（幂等会拦住），不可丢失。
		CommitInterval: 0,
	})
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	w.tasks.Run(ctx, func(runCtx context.Context) { w.batch.Start(runCtx) })

	for {
		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				w.batch.Stop() // Stop 内部会做最后一次 flush，从而提交残留 offset
				return ctx.Err()
			}
			applog.C(ctx).Errorw("kafka_fetch_failed", logx.Field(applog.KeyErr, err))
			continue
		}

		row, evt, err := w.parseMessage(msg.Value)
		msgCtx := applog.ContextFromKafka(ctx, msg, evt.TraceID, evt.RoundID, evt.MerchantID, evt.GameCode, evt.UserID)

		if err != nil {
			// 解析失败的消息永远无法处理：提交 offset 跳过，避免卡住分区。
			// 这是有意为之的取舍，因此日志必须是 error 级别以便告警发现。
			applog.C(msgCtx).Errorw("kafka_parse_failed_skipped",
				logx.Field(applog.KeyErr, err),
				logx.Field("topic", msg.Topic),
				logx.Field("offset", msg.Offset),
			)
			_ = w.reader.CommitMessages(ctx, msg)
			continue
		}

		if !w.claimIdempotency(msgCtx, row.EventID) {
			// 已处理过（outbox 的 at-least-once 会带来重投）：直接提交 offset 跳过。
			_ = w.reader.CommitMessages(msgCtx, msg)
			continue
		}

		if w.svcCtx.Config.RtpWatch.Enabled {
			w.evaluateRtp(msgCtx, evt)
		}

		// 注意：这里**不提交 offset**。提交动作发生在 flush 成功之后。
		if err := w.batch.Add(msgCtx, batchItem{row: row, msg: msg}); err != nil {
			applog.C(msgCtx).Errorw("batch_add_failed", logx.Field(applog.KeyErr, err))
		}
	}
}

// claimIdempotency 用 processed_events 做消费端兜底幂等。
//
// 与 outbox 的投递语义配合：投递是 at-least-once，重复消息在这里被拦住。
// eventID 为空（旧格式消息）时不做去重，交由 ClickHouse 侧的幂等键兜底。
func (w *Worker) claimIdempotency(ctx context.Context, eventID string) bool {
	if eventID == "" || w.svcCtx.Idem == nil {
		return true
	}
	claimed := false
	err := w.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		ok, err := w.svcCtx.Idem.AcquireInTx(ctx, session, consumerGroup, eventID)
		if err != nil {
			return err
		}
		if !ok {
			return outbox.ErrDuplicateEvent
		}
		claimed = true
		return nil
	})
	if err != nil {
		if errors.Is(err, outbox.ErrDuplicateEvent) {
			applog.C(ctx).Infow("duplicate_event_skipped", logx.Field("eventId", eventID))
			return false
		}
		// 数据库抖动：不认领，让消息保持未提交状态、稍后重投。
		// 返回 false 会提交 offset，因此这里必须返回 true 并继续，
		// 由 flush 阶段再兜一次幂等。选择日志告警而非丢消息。
		applog.C(ctx).Errorw("idempotency_check_failed", logx.Field(applog.KeyErr, err), logx.Field("eventId", eventID))
		return true
	}
	return claimed
}

func (w *Worker) evaluateRtp(ctx context.Context, evt kafka.RoundSettledEvent) {
	alerts, err := w.svcCtx.Recorder.Record(ctx, rtpwatchdog.RecordInput{
		MerchantCode: evt.MerchantID,
		GameCode:     evt.GameCode,
		UserID:       evt.UserID,
		BetMinor:     evt.BetAmount,
		WinMinor:     evt.WinAmount,
	})
	if err != nil {
		applog.C(ctx).Errorw("rtp_recorder_failed", logx.Field(applog.KeyErr, err))
		return
	}
	for _, alert := range alerts {
		if err := w.svcCtx.Enforcer.Handle(ctx, alert); err != nil {
			applog.C(ctx).Errorw("rtp_enforcer_failed", logx.Field(applog.KeyErr, err))
		}
	}
}

// parseMessage 支持两种消息体：
//   - outbox 信封（新）：{eventId,topic,payload,...}，从中取出 payload 再解析；
//   - 裸 RoundSettledEvent（旧）：直接解析，保证滚动升级期间不丢数据。
func (w *Worker) parseMessage(raw []byte) (clickhouse.RoundSettledRow, kafka.RoundSettledEvent, error) {
	var evt kafka.RoundSettledEvent

	payload := raw
	traceFromEnvelope := ""
	var env envelopeItem
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Payload) > 0 {
		payload = env.Payload
		traceFromEnvelope = env.TraceID
	} else if w.svcCtx.Config.Kafka.RequireEnvelope {
		return clickhouse.RoundSettledRow{}, evt, errors.New("message is not an outbox envelope (Kafka.RequireEnvelope=true)")
	}

	if err := json.Unmarshal(payload, &evt); err != nil {
		return clickhouse.RoundSettledRow{}, evt, err
	}
	// 信封里的 trace 优先级更高：它由投递方写入，跨进程更可靠
	if evt.TraceID == "" {
		evt.TraceID = traceFromEnvelope
	}

	merchantID, err := w.resolveMerchantID(evt.MerchantID)
	if err != nil {
		return clickhouse.RoundSettledRow{}, evt, err
	}

	return clickhouse.RoundSettledRow{
		EventID:      evt.EventID,
		TraceID:      evt.TraceID,
		RoundID:      evt.RoundID,
		UserID:       evt.UserID,
		MerchantID:   merchantID,
		GameCode:     evt.GameCode,
		BetAmount:    evt.BetAmount,
		WinAmount:    evt.WinAmount,
		Multiplier:   evt.Multiplier,
		RtpTier:      evt.RtpTier,
		BalanceAfter: evt.Balance,
		SettledAt:    evt.SettledAt,
	}, evt, nil
}

func (w *Worker) resolveMerchantID(merchantIDOrCode string) (uint64, error) {
	if id, err := strconv.ParseUint(merchantIDOrCode, 10, 64); err == nil {
		return id, nil
	}

	merchant, err := w.svcCtx.Merchants.FindOneByMerchantCode(context.Background(), merchantIDOrCode)
	if err != nil {
		return 0, err
	}
	return merchant.Id, nil
}

// flush 批量写入 ClickHouse，**写成功后才提交本批 offset**。
//
// 失败时不提交：消息会被重新投递（at-least-once），重复由 processed_events 拦住。
func (w *Worker) flush(ctx context.Context, items []batchItem) error {
	if len(items) == 0 {
		return nil
	}
	rows := make([]clickhouse.RoundSettledRow, 0, len(items))
	msgs := make([]kafkago.Message, 0, len(items))
	for _, it := range items {
		rows = append(rows, it.row)
		msgs = append(msgs, it.msg)
	}

	applog.C(ctx).Infow("clickhouse_flush", logx.Field("rows", len(rows)))
	if err := w.svcCtx.Writer.BatchInsertRoundSettled(ctx, rows); err != nil {
		applog.C(ctx).Errorw("clickhouse_batch_insert_failed",
			logx.Field(applog.KeyErr, err),
			logx.Field("rows", len(rows)),
			logx.Field("note", "offset 未提交，本批将重投"),
		)
		return err
	}

	if err := w.reader.CommitMessages(ctx, msgs...); err != nil {
		// 落库成功但提交失败：下次会重投这一批，由幂等拦住，
		// 因此这里只需告警，不需要回滚 ClickHouse（也无法回滚）。
		applog.C(ctx).Errorw("kafka_commit_failed_after_flush",
			logx.Field(applog.KeyErr, err),
			logx.Field("rows", len(rows)),
		)
	}
	return nil
}

func (w *Worker) Close() error {
	// 先停 Kafka reader（fetch 循环退出后 Run 会调用 batch.Stop 做最后一次
	// flush 并提交 offset），再等批量循环收尾，避免刷盘写到一半进程就退了。
	var readerErr error
	if w.reader != nil {
		readerErr = w.reader.Close()
	}
	drainCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := w.tasks.Shutdown(drainCtx); err != nil {
		applog.C(context.Background()).Errorw("batch_loop_shutdown_failed", logx.Field(applog.KeyErr, err))
	}
	return readerErr
}
