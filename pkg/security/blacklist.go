package security

import (
	"context"
	"fmt"
	"time"

	"fastgame/internal/model"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
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

func (b *Blacklist) CheckAccess(ctx context.Context, ip, merchantID, userID string) error {
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

	if len(userID) > 0 {
		if blocked, err := b.IsBlocked(ctx, model.BlacklistTypeUserID, userID); err != nil {
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
	return b.Reconcile(ctx, items)
}

// Reconcile 以数据库为准做全量对账：
//
//  1. 库里的每一行同步到 Redis（有效则封禁、失效或 status!=1 则解封）；
//  2. **删除库里已经不存在的 Redis 键**。
//
// 第 2 步是必需的：只做"遍历库里的行"是只增不减的单向同步，
// 直接从数据库删掉/改掉一行（运维手工处理时很常见）会留下永久封禁的
// 孤儿键（TTL=-1），被封的用户再也无法下注，而库里查不到任何依据。
//
// 只清理 blacklist:<listType>:<value> 形式的键，不会碰到其他业务键。
func (b *Blacklist) Reconcile(ctx context.Context, items []*model.RiskBlacklist) error {
	expected := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == nil || item.ListValue == "" {
			continue
		}
		expected[blacklistKey(item.ListType, item.ListValue)] = struct{}{}
		if err := b.SyncItem(ctx, item); err != nil {
			return err
		}
	}

	live, err := b.keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range live {
		if _, ok := expected[key]; ok {
			continue
		}
		if err := b.client.Del(ctx, key).Err(); err != nil {
			return err
		}
		logx.Infof("[BLACKLIST] 清理库中已不存在的孤儿封禁键: %s", key)
	}
	return nil
}

// keys 用 SCAN 枚举所有 blacklist:* 键（不用 KEYS，避免在大 key 空间下阻塞 Redis）。
func (b *Blacklist) keys(ctx context.Context) ([]string, error) {
	var (
		cursor uint64
		out    []string
	)
	for {
		batch, next, err := b.client.Scan(ctx, cursor, "blacklist:*", 200).Result()
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if next == 0 {
			return out, nil
		}
		cursor = next
	}
}
