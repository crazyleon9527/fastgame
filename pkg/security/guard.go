package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/ratelimit"
	"fastgame/pkg/validator"

	"github.com/redis/go-redis/v9"
)

const (
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSignature = "X-Signature"
)

type Config struct {
	SkipSignVerify       bool
	TimestampWindow      time.Duration
	UserBetLimitPerMin   int
	IPLimitPerSec        int
	MinResponseDelay     time.Duration
}

type Guard struct {
	cfg      Config
	merchants model.MerchantsModel
	replay   *ReplayGuard
	limiter  *ratelimit.Limiter
}

func NewGuard(cfg Config, merchants model.MerchantsModel, redis *redis.Client) *Guard {
	if cfg.TimestampWindow <= 0 {
		cfg.TimestampWindow = 60 * time.Second
	}
	if cfg.UserBetLimitPerMin <= 0 {
		cfg.UserBetLimitPerMin = 60
	}
	if cfg.IPLimitPerSec <= 0 {
		cfg.IPLimitPerSec = 20
	}
	return &Guard{
		cfg:       cfg,
		merchants: merchants,
		replay:    NewReplayGuard(redis, cfg.TimestampWindow, 5*time.Minute),
		limiter:   ratelimit.NewLimiter(redis),
	}
}

type BetCheckInput struct {
	Method     string
	Path       string
	Body       string
	MerchantID string
	UserID     uint64
	ClientIP   string
	Headers    http.Header
}

func (g *Guard) CheckBet(ctx context.Context, in BetCheckInput, limits validator.BetLimits, betAmount float64) error {
	if err := validator.BetLimits(limits).Validate(betAmount); err != nil {
		return err
	}

	if in.ClientIP != "" {
		ok, err := g.limiter.Allow(ctx, "ip:"+in.ClientIP, g.cfg.IPLimitPerSec, time.Second)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("ip rate limit exceeded")
		}
	}

	ok, err := g.limiter.Allow(ctx, fmt.Sprintf("user:%d", in.UserID), g.cfg.UserBetLimitPerMin, time.Minute)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user rate limit exceeded")
	}

	if g.cfg.SkipSignVerify {
		return nil
	}

	ts := in.Headers.Get(HeaderTimestamp)
	nonce := in.Headers.Get(HeaderNonce)
	sig := in.Headers.Get(HeaderSignature)
	if err := g.replay.Validate(ctx, ts, nonce); err != nil {
		return err
	}

	merchant, err := g.merchants.FindOneByMerchantCode(ctx, in.MerchantID)
	if err != nil {
		return fmt.Errorf("merchant not found")
	}
	if !merchant.PrivateKey.Valid || merchant.PrivateKey.String == "" {
		return fmt.Errorf("merchant secret not configured")
	}

	payload := BuildSignPayload(in.Method, in.Path, in.Body, ts, nonce)
	return VerifySign(merchant.PrivateKey.String, payload, sig)
}

func ParseBetLimits(raw map[string]json.RawMessage) validator.BetLimits {
	limits := validator.BetLimits{Min: 0.01, Max: 10000}
	if raw == nil {
		return limits
	}
	data, ok := raw["bet_limits"]
	if !ok {
		return limits
	}
	_ = json.Unmarshal(data, &limits)
	if limits.Min <= 0 {
		limits.Min = 0.01
	}
	if limits.Max <= 0 {
		limits.Max = 10000
	}
	return limits
}

func NormalizeResponseDelay(start time.Time, minDelay time.Duration) {
	if minDelay <= 0 {
		return
	}
	elapsed := time.Since(start)
	if elapsed < minDelay {
		time.Sleep(minDelay - elapsed)
	}
}
