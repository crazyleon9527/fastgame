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

const TopicLedgerPosted = "game.ledger.posted"

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

type OutboxSink struct {
	store  *outbox.Store
	source string
}

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

	env, err := outbox.NewEnvelopeWithID(
		outbox.EventTopic(TopicLedgerPosted), s.source, trace.ID(ctx), entry.TransactionId, evt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("构建账变事件信封失败: %w", err)
	}

	// 多租户隔离：分区键使用 merchantCode:userId，保证多商户同名玩家各自保序不混淆
	partitionKey := fmt.Sprintf("%s:%s", entry.MerchantCode, entry.UserId)
	if err := s.store.InsertInTx(ctx, session, &outbox.Record{
		ID:           entry.TransactionId,
		Topic:        TopicLedgerPosted,
		MerchantID:   entry.MerchantId,
		PartitionKey: partitionKey,
		Payload:      env,
	}); err != nil {
		return fmt.Errorf("登记账变事件失败: %w", err)
	}
	return nil
}
