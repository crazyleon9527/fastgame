package security

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fastgame/internal/model"

	"github.com/redis/go-redis/v9"
)

type IPWhitelist struct {
	merchants model.MerchantsModel
	redis     *redis.Client
	ttl       time.Duration
}

func NewIPWhitelist(merchants model.MerchantsModel, redis *redis.Client) *IPWhitelist {
	return &IPWhitelist{
		merchants: merchants,
		redis:     redis,
		ttl:       10 * time.Minute,
	}
}

func (w *IPWhitelist) cacheKey(merchantCode string) string {
	return fmt.Sprintf("merchant:allowips:%s", merchantCode)
}

func (w *IPWhitelist) Check(ctx context.Context, merchantCode, clientIP string) error {
	if merchantCode == "" || clientIP == "" {
		return nil
	}

	allowed, err := w.loadAllowedIPs(ctx, merchantCode)
	if err != nil {
		return err
	}
	if len(allowed) == 0 {
		return nil
	}

	for _, ip := range allowed {
		if ip == clientIP {
			return nil
		}
	}
	return fmt.Errorf("merchant ip not whitelisted")
}

func (w *IPWhitelist) loadAllowedIPs(ctx context.Context, merchantCode string) ([]string, error) {
	key := w.cacheKey(merchantCode)
	raw, err := w.redis.Get(ctx, key).Result()
	if err == nil {
		var ips []string
		if err := json.Unmarshal([]byte(raw), &ips); err == nil {
			return ips, nil
		}
	}
	if err != nil && err != redis.Nil {
		return nil, err
	}

	ips, err := w.merchants.FindAllowedIPs(ctx, merchantCode)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	if data, err := json.Marshal(ips); err == nil {
		_ = w.redis.Set(ctx, key, data, w.ttl).Err()
	}
	return ips, nil
}

func (w *IPWhitelist) Invalidate(ctx context.Context, merchantCode string) error {
	return w.redis.Del(ctx, w.cacheKey(merchantCode)).Err()
}
