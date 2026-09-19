package ledger

import (
	"context"
	"fmt"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/outbox"
	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TopicLedgerPosted 账变事件 topic。
//
// 目前还没有消费方——它是给后续「结算/对账/商户通知」阶段准备的：
// 账变是资金事实，下游要按事件流做账单与差异检测，而不是定时扫全表。
// 在此之前事件由 outbox 可靠投递到 Kafka（不会丢），消费端接上即可。
const TopicLedgerPosted = "game.ledger.posted"

// LedgerPostedEvent 账变事件载荷
type LedgerPostedEvent struct {
	TransactionID string `json:"transactionId"`
	TxType        string `json:"txType"`
	Direction     string `json:"direction"`
	MerchantID    uint64 `json:"merchantId"`
	MerchantCode  string `json:"merchantCode"`
	UserID        string `json:"userId"`
	GameCode      string `json:"gameCode"`
	RoundID       string `json:"roundId,omitempty"`
	Currency      string `json:"currency"`
	Amount        int64  `json:"amount"`
	BalanceBefore int64  `json:"balanceBefore"`
	BalanceAfter  int64  `json:"balanceAfter"`
	DriftMinor    int64  `json:"driftMinor"`
	Status        string `json:"status"`
	ExternalTxID  string `json:"externalTxId,omitempty"`
	OccurredAt    int64  `json:"occurredAt"`
}

// OutboxSink 把账变事件登记进事务性发件箱。
//
// 与业务写同事务提交，因此"账变了但事件没发出去"不可能发生——
// 投递由 outbox.Dispatcher 负责（失败退避重试）。
type OutboxSink struct {
	store  *outbox.Store
	source string
}

// NewOutboxSink source 是事件来源标识（服务名，便于消费端区分）。
func NewOutboxSink(store *outbox.Store, source string) *OutboxSink {
	return &OutboxSink{store: store, source: source}
}

func (s *OutboxSink) PublishInTx(ctx context.Context, session sqlx.Session, entry *model.GameTransaction) error {
	if s == nil || s.store == nil || entry == nil {
		return nil
	}

	evt := LedgerPostedEvent{
		TransactionID: entry.TransactionId,
		TxType:        entry.TxType,
		Direction:     entry.Direction,
		MerchantID:    entry.MerchantId,
		MerchantCode:  entry.MerchantCode,
		UserID:        entry.UserId,
		GameCode:      entry.GameCode,
		Currency:      entry.Currency,
		Amount:        entry.Amount,
		BalanceBefore: entry.BalanceBefore,
		BalanceAfter:  entry.BalanceAfter,
		DriftMinor:    entry.DriftMinor,
		Status:        entry.Status,
		ExternalTxID:  entry.ExternalTxId,
		OccurredAt:    entry.CreatedAt.UnixMilli(),
	}
	if entry.RoundId.Valid {
		evt.RoundID = entry.RoundId.String
	}
	if evt.OccurredAt == 0 {
		evt.OccurredAt = time.Now().UTC().UnixMilli()
	}

	// 事件 ID 复用账变流水号：两边同一个 ID，排查时不必再做映射。
	env, err := outbox.NewEnvelopeWithID(
		outbox.EventTopic(TopicLedgerPosted), s.source, trace.ID(ctx), entry.TransactionId, evt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("构建账变事件信封失败: %w", err)
	}

	// 分区键用 user_id：同一玩家的账变进同一分区，消费端能按玩家保序。
	if err := s.store.InsertInTx(ctx, session, &outbox.Record{
		ID:           entry.TransactionId,
		Topic:        TopicLedgerPosted,
		MerchantID:   entry.MerchantId,
		PartitionKey: entry.UserId,
		Payload:      env,
	}); err != nil {
		return fmt.Errorf("登记账变事件失败: %w", err)
	}
	return nil
}
