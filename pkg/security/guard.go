package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/money"
	"fastgame/pkg/validator"

	"github.com/redis/go-redis/v9"
)

const (
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSignature = "X-Signature"
)

type Config struct {
	SkipMerchantSign    bool
	SkipSessionEnvelope bool
	// SkipSignVerify 兼容旧配置：为 true 时同时跳过商户签名与会话 Envelope
	SkipSignVerify      bool
	TimestampWindow     time.Duration
	MaxClockSkew        time.Duration
	MinResponseDelay    time.Duration
}

func (c Config) skipMerchant() bool {
	return c.SkipMerchantSign || c.SkipSignVerify
}

func (c Config) skipEnvelope() bool {
	return c.SkipSessionEnvelope || c.SkipSignVerify
}

type Guard struct {
	cfg       Config
	merchants model.MerchantsModel
	replay    *ReplayGuard
	blacklist *Blacklist
	suspend   *SuspendStore
	whitelist *IPWhitelist
	waf       *WAF
}

func NewGuard(cfg Config, merchants model.MerchantsModel, redis *redis.Client) *Guard {
	if cfg.TimestampWindow <= 0 {
		cfg.TimestampWindow = 60 * time.Second
	}
	maxSkew := cfg.MaxClockSkew
	if maxSkew <= 0 {
		maxSkew = 5 * time.Second
	}
	return &Guard{
		cfg:       cfg,
		merchants: merchants,
		replay:    NewReplayGuard(redis, cfg.TimestampWindow, maxSkew, 5*time.Minute),
		blacklist: NewBlacklist(redis),
		suspend:   NewSuspendStore(redis),
		whitelist: NewIPWhitelist(merchants, redis),
		waf:       NewWAF(),
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

func (g *Guard) CheckRequest(r *http.Request, body string) error {
	return g.waf.InspectRequest(r, body)
}

func (g *Guard) CheckAccess(ctx context.Context, clientIP, merchantID string, userID uint64) error {
	if err := g.blacklist.CheckAccess(ctx, clientIP, merchantID, userID); err != nil {
		return err
	}
	suspended, err := g.suspend.IsUserSuspended(ctx, userID)
	if err != nil {
		return err
	}
	if suspended {
		return fmt.Errorf("user suspended pending audit")
	}
	return g.whitelist.Check(ctx, merchantID, clientIP)
}

func (g *Guard) CheckGameFlagged(ctx context.Context, merchantID, gameCode string) error {
	flagged, err := g.suspend.IsGameFlagged(ctx, merchantID, gameCode)
	if err != nil {
		return err
	}
	if flagged {
		return fmt.Errorf("game flagged pending audit")
	}
	return nil
}

func (g *Guard) VerifySessionEnvelope(dynamicKey, roundID, action, timestamp, signature string) error {
	if g.cfg.skipEnvelope() {
		return nil
	}
	return VerifyEnvelope(dynamicKey, roundID, action, timestamp, signature)
}

func (g *Guard) CheckBet(ctx context.Context, in BetCheckInput, limits validator.BetLimits, betAmountMinor int64) error {
	if err := g.CheckAccess(ctx, in.ClientIP, in.MerchantID, in.UserID); err != nil {
		return err
	}

	if err := validator.BetLimits(limits).Validate(money.AmountFromMinor(betAmountMinor)); err != nil {
		return err
	}

	if g.cfg.skipMerchant() {
		return nil
	}

	ts := in.Headers.Get(HeaderTimestamp)
	nonce := in.Headers.Get(HeaderNonce)
	sig := in.Headers.Get(HeaderSignature)
	if err := g.replay.Validate(ctx, ts, nonce); err != nil {
		return err
	}

	secrets, err := g.merchants.FindSecretsByMerchantCode(ctx, in.MerchantID)
	if err != nil {
		return fmt.Errorf("merchant not found")
	}

	payload := BuildSignPayload(in.Method, in.Path, in.Body, ts, nonce)
	return VerifyMerchantSign(secrets, payload, sig)
}

func ParseBetLimits(raw map[string]json.RawMessage) validator.BetLimits {
	limits := validator.BetLimits{Min: money.Scale / 100, Max: 10000 * money.Scale}
	if raw == nil {
		return limits
	}
	data, ok := raw["bet_limits"]
	if !ok {
		return limits
	}
	_ = json.Unmarshal(data, &limits)
	if limits.Min <= 0 {
		limits.Min = money.Scale / 100
	}
	if limits.Max <= 0 {
		limits.Max = 10000 * money.Scale
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
