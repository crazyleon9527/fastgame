package security

import (
	"context"
	"fmt"
	"time"

	"fastgame/internal/model"

	"github.com/redis/go-redis/v9"
)

type Blacklist struct {
	client *redis.Client
}

func NewBlacklist(client *redis.Client) *Blacklist {
	return &Blacklist{client: client}
}

func blacklistKey(listType, value string) string {
	return fmt.Sprintf("blacklist:%s:%s", listType, value)
}

func (b *Blacklist) Block(ctx context.Context, listType, value string, ttl time.Duration) error {
	if ttl <= 0 {
		return b.client.Set(ctx, blacklistKey(listType, value), "1", 0).Err()
	}
	return b.client.Set(ctx, blacklistKey(listType, value), "1", ttl).Err()
}

func (b *Blacklist) Unblock(ctx context.Context, listType, value string) error {
	return b.client.Del(ctx, blacklistKey(listType, value)).Err()
}

func (b *Blacklist) IsBlocked(ctx context.Context, listType, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	n, err := b.client.Exists(ctx, blacklistKey(listType, value)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (b *Blacklist) CheckAccess(ctx context.Context, ip, merchantID string, userID uint64) error {
	if blocked, err := b.IsBlocked(ctx, model.BlacklistTypeIP, ip); err != nil {
		return err
	} else if blocked {
		return fmt.Errorf("ip blocked")
	}

	if blocked, err := b.IsBlocked(ctx, model.BlacklistTypeMerchant, merchantID); err != nil {
		return err
	} else if blocked {
		return fmt.Errorf("merchant blocked")
	}

	if userID > 0 {
		if blocked, err := b.IsBlocked(ctx, model.BlacklistTypeUserID, fmt.Sprintf("%d", userID)); err != nil {
			return err
		} else if blocked {
			return fmt.Errorf("user blocked")
		}
	}
	return nil
}

func (b *Blacklist) SyncItem(ctx context.Context, item *model.RiskBlacklist) error {
	if item.Status != 1 {
		return b.Unblock(ctx, item.ListType, item.ListValue)
	}

	var ttl time.Duration
	if item.ExpiresAt.Valid {
		remaining := time.Until(item.ExpiresAt.Time)
		if remaining <= 0 {
			return b.Unblock(ctx, item.ListType, item.ListValue)
		}
		ttl = remaining
	}
	return b.Block(ctx, item.ListType, item.ListValue, ttl)
}

func (b *Blacklist) SyncAll(ctx context.Context, items []*model.RiskBlacklist) error {
	for _, item := range items {
		if err := b.SyncItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
