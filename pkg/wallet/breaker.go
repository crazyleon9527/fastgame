package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fastgame/pkg/money"

	"github.com/zeromicro/go-zero/core/breaker"
	"github.com/zeromicro/go-zero/core/logx"
)

// BreakerConfig 商户级熔断：慢调用 (>500ms) 视为失败，快速熔断保护协程池
type BreakerConfig struct {
	SlowThreshold time.Duration
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
		cfg = DefaultBreakerConfig()
	}
	return &breakerClient{inner: inner, cfg: cfg}
}

func (c *breakerClient) breakerKey(merchantID string) string {
	return "wallet:" + merchantID
}

func (c *breakerClient) run(ctx context.Context, merchantID string, op string, fn func() error) error {
	brk := breaker.GetBreaker(c.breakerKey(merchantID))
	start := time.Now()
	err := brk.Do(func() error {
		if err := fn(); err != nil {
			return err
		}
		if elapsed := time.Since(start); elapsed > c.cfg.SlowThreshold {
			logx.WithContext(ctx).Slowf("wallet slow call: merchant=%s op=%s elapsed=%s", merchantID, op, elapsed)
			return fmt.Errorf("%w: %s took %s", ErrSlowResponse, op, elapsed)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, breaker.ErrServiceUnavailable) {
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
