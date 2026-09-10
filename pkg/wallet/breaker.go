package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fastgame/pkg/money"

	applog "fastgame/pkg/log"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/breaker"

	"github.com/zeromicro/go-zero/core/logx"
)

type BreakerConfig struct {
	SlowThreshold time.Duration
	Redis         *redis.Client
}

func DefaultBreakerConfig() BreakerConfig {
	return BreakerConfig{SlowThreshold: 500 * time.Millisecond}
}

type breakerClient struct {
	inner Client
	cfg   BreakerConfig
}

func NewBreakerClient(inner Client, cfg BreakerConfig) Client {
	if cfg.SlowThreshold <= 0 {
		cfg.SlowThreshold = DefaultBreakerConfig().SlowThreshold
	}
	return &breakerClient{inner: inner, cfg: cfg}
}

func (c *breakerClient) breakerKey(merchantID string) string {
	return "wallet:" + merchantID
}

func (c *breakerClient) overrideKey(merchantID string) string {
	return "wallet:breaker:override:" + merchantID
}

func (c *breakerClient) stateKey(merchantID string) string {
	return "wallet:breaker:open:" + merchantID
}

// ResetBreaker 运维手动解除熔断（设置 override + 清除 open 标记）
func ResetBreaker(ctx context.Context, rdb *redis.Client, merchantID string, ttl time.Duration) error {
	if rdb == nil || merchantID == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	pipe := rdb.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("wallet:breaker:override:%s", merchantID), "1", ttl)
	pipe.Del(ctx, fmt.Sprintf("wallet:breaker:open:%s", merchantID))
	_, err := pipe.Exec(ctx)
	if err == nil {
		SetBreakerMetric(merchantID, false)
	}
	return err
}

func (c *breakerClient) isOverridden(ctx context.Context, merchantID string) bool {
	if c.cfg.Redis == nil {
		return false
	}
	n, err := c.cfg.Redis.Exists(ctx, c.overrideKey(merchantID)).Result()
	return err == nil && n > 0
}

func (c *breakerClient) markOpen(ctx context.Context, merchantID string) {
	if c.cfg.Redis == nil {
		return
	}
	_ = c.cfg.Redis.Set(ctx, c.stateKey(merchantID), time.Now().Unix(), breakerOpenTTL()).Err()
	SetBreakerMetric(merchantID, true)
}

func (c *breakerClient) run(ctx context.Context, merchantID string, op string, fn func() error) error {
	if c.isOverridden(ctx, merchantID) {
		return fn()
	}

	brk := breaker.GetBreaker(c.breakerKey(merchantID))
	start := time.Now()
	err := brk.Do(func() error {
		if err := fn(); err != nil {
			return err
		}
		if elapsed := time.Since(start); elapsed > c.cfg.SlowThreshold {
			applog.C(ctx).Sloww("wallet_slow_call",
				logx.Field(applog.KeyMerchantID, merchantID),
				logx.Field("op", op),
				logx.Field(applog.KeyDurationMs, elapsed.Milliseconds()),
			)
			return fmt.Errorf("%w: %s took %s", ErrSlowResponse, op, elapsed)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, breaker.ErrServiceUnavailable) {
			c.markOpen(ctx, merchantID)
			return ErrCircuitOpen
		}
		return err
	}
	return nil
}

func (c *breakerClient) GetBalance(ctx context.Context, merchantID string, userID uint64) (money.Amount, error) {
	var balance money.Amount
	err := c.run(ctx, merchantID, "GetBalance", func() error {
		var err error
		balance, err = c.inner.GetBalance(ctx, merchantID, userID)
		return err
	})
	return balance, err
}

func (c *breakerClient) Bet(ctx context.Context, req BetReq) (*Result, error) {
	var result *Result
	err := c.run(ctx, req.MerchantID, "Bet", func() error {
		var err error
		result, err = c.inner.Bet(ctx, req)
		return err
	})
	return result, err
}

func (c *breakerClient) Win(ctx context.Context, req WinReq) (*Result, error) {
	var result *Result
	err := c.run(ctx, req.MerchantID, "Win", func() error {
		var err error
		result, err = c.inner.Win(ctx, req)
		return err
	})
	return result, err
}

func (c *breakerClient) Rollback(ctx context.Context, req RollbackReq) error {
	return c.run(ctx, req.MerchantID, "Rollback", func() error {
		return c.inner.Rollback(ctx, req)
	})
}

func (c *breakerClient) CheckTransaction(ctx context.Context, merchantID string, userID uint64, roundID string) (*TxCheckResult, error) {
	var result *TxCheckResult
	err := c.run(ctx, merchantID, "CheckTransaction", func() error {
		var err error
		result, err = c.inner.CheckTransaction(ctx, merchantID, userID, roundID)
		return err
	})
	return result, err
}
