package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/gameconfig"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/money"
	"fastgame/pkg/ratelimit"
	"fastgame/pkg/security"
	"fastgame/pkg/session"
	"fastgame/pkg/wallet"
	"fastgame/services/rgs/internal/config"
	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	zeroredis "github.com/zeromicro/go-zero/core/stores/redis"
)

type stubGameConfig struct{}

type noopPendingTx struct{}

func (noopPendingTx) Insert(context.Context, *model.PendingTransaction) error { return nil }
func (noopPendingTx) MarkSettled(context.Context, string) error               { return nil }
func (noopPendingTx) MarkWinPending(context.Context, string, int64, string) error {
	return nil
}
func (noopPendingTx) ListStalePending(context.Context, time.Duration, int) ([]*model.PendingTransaction, error) {
	return nil, nil
}
func (noopPendingTx) FindByTraceID(context.Context, string) ([]*model.PendingTransaction, error) {
	return nil, nil
}
func (noopPendingTx) FindByRoundID(context.Context, string) (*model.PendingTransaction, error) {
	return nil, model.ErrNotFound
}
func (noopPendingTx) MarkDone(context.Context, uint64) error            { return nil }
func (noopPendingTx) MarkFailed(context.Context, uint64, string) error  { return nil }
func (noopPendingTx) IncrementRetry(context.Context, uint64, string) error { return nil }

type noopPendingOps struct{}

func (noopPendingOps) Insert(context.Context, *model.WalletPendingOp) (sql.Result, error) {
	return nil, nil
}
func (noopPendingOps) FindByRoundOp(context.Context, string, string) (*model.WalletPendingOp, error) {
	return nil, model.ErrNotFound
}
func (noopPendingOps) ListPending(context.Context, int) ([]*model.WalletPendingOp, error) {
	return nil, nil
}
func (noopPendingOps) MarkDone(context.Context, uint64) error           { return nil }
func (noopPendingOps) MarkFailed(context.Context, uint64, string) error { return nil }
func (noopPendingOps) IncrementRetry(context.Context, uint64, string) error { return nil }

type noopReplayStore struct{}

func (noopReplayStore) Insert(context.Context, *model.GameRoundReplay) error { return nil }
func (noopReplayStore) FindByRoundID(context.Context, string) (*model.GameRoundReplay, error) {
	return nil, model.ErrNotFound
}

func (stubGameConfig) Load(_ context.Context, _, _ string) (*gameconfig.Config, error) {
	return &gameconfig.Config{
		RtpTier: "default",
		Raw: map[string]json.RawMessage{
			"bet_limits": json.RawMessage(`{"min":10000,"max":10000000,"allowed":[100000]}`),
		},
	}, nil
}

func newIntegrationSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	_ = rdb.Set(ctx, "merchant:allowips:m001", "[]", time.Hour).Err()

	zeroRedis := zeroredis.MustNewRedis(zeroredis.RedisConf{Host: mr.Addr(), Type: "node"})

	return &svc.ServiceContext{
		Config: config.Config{
			Security: config.SecurityConf{
				SkipMerchantSign:    true,
				SkipSessionEnvelope: false,
				TimestampWindowSec:  120,
				MaxClockSkewSec:     5,
				UserLimitPerSec:     1000,
				IPLimitPerSec:       1000,
			},
			Session: config.SessionConf{TTLHours: 24},
		},
		Redis:      rdb,
		Lock:       lock.NewRedisLock(rdb),
		Idempotent: idempotent.NewStore(rdb, time.Hour),
		Wallet:     wallet.NewMockClient(money.FromMajor(10000)),
		GameConfig: stubGameConfig{},
		Kafka:      kafka.NewProducer([]string{"127.0.0.1:1"}),
		Guard: security.NewGuard(security.Config{
			SkipMerchantSign:    true,
			SkipSessionEnvelope: false,
			TimestampWindow:     120 * time.Second,
			MaxClockSkew:        5 * time.Second,
		}, nil, rdb),
		RateLimit:   ratelimit.NewGateway(zeroRedis, 1000, 1000, 1000, 1000),
		Session:     session.NewStore(rdb, time.Hour),
		PendingTx:   noopPendingTx{},
		PendingOps:  noopPendingOps{},
		ReplayStore: noopReplayStore{},
	}
}

func TestBetGoldenPathSessionEnvelope(t *testing.T) {
	svcCtx := newIntegrationSvcCtx(t)

	// 1) Create session
	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001",
		UserId:     10001,
		GameCode:   "fishing",
		ClientSeed: "client-test-seed",
	})
	sreq := httptest.NewRequest(http.MethodPost, "/api/v1/game/session", bytes.NewReader(sessionBody))
	sreq.Header.Set("Content-Type", "application/json")
	sreq.RemoteAddr = "127.0.0.1:1234"
	sw := httptest.NewRecorder()
	SessionHandler(svcCtx)(sw, sreq)
	if sw.Code != http.StatusOK {
		t.Fatalf("session status=%d body=%s", sw.Code, sw.Body.String())
	}
	var sessionResp types.SessionResp
	if err := json.Unmarshal(sw.Body.Bytes(), &sessionResp); err != nil {
		t.Fatal(err)
	}
	if sessionResp.DynamicSessionKey == "" || sessionResp.SessionToken == "" {
		t.Fatal("missing session credentials")
	}

	// 2) Bet with secure envelope (prod-like)
	roundID := "integration-round-1"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	envSig := security.SignEnvelope(sessionResp.DynamicSessionKey, roundID, "cast", ts)

	betBody, _ := json.Marshal(types.BetReq{
		MerchantId:   "m001",
		UserId:       10001,
		SessionToken: sessionResp.SessionToken,
		GameCode:     "fishing",
		Action:       "cast",
		BetAmount:    100000,
		RoundId:      roundID,
		SequenceId:   sessionResp.NextSequenceId,
	})
	breq := httptest.NewRequest(http.MethodPost, "/api/v1/game/bet", bytes.NewReader(betBody))
	breq.Header.Set("Content-Type", "application/json")
	breq.Header.Set(security.HeaderTimestamp, ts)
	breq.Header.Set(security.HeaderSessionSign, envSig)
	breq.RemoteAddr = "127.0.0.1:1234"
	bw := httptest.NewRecorder()
	BetHandler(svcCtx)(bw, breq)
	if bw.Code != http.StatusOK && bw.Code != http.StatusAccepted {
		t.Fatalf("bet status=%d body=%s", bw.Code, bw.Body.String())
	}

	var betResp types.BetResp
	if err := json.Unmarshal(bw.Body.Bytes(), &betResp); err != nil {
		t.Fatal(err)
	}
	if betResp.RoundId != roundID {
		t.Fatalf("round mismatch: %s", betResp.RoundId)
	}
	if betResp.Balance <= 0 {
		t.Fatalf("expected positive balance, got %d", betResp.Balance)
	}
}

func TestBetRejectsInvalidEnvelope(t *testing.T) {
	svcCtx := newIntegrationSvcCtx(t)

	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001",
		UserId:     10001,
		GameCode:   "fishing",
	})
	sreq := httptest.NewRequest(http.MethodPost, "/api/v1/game/session", bytes.NewReader(sessionBody))
	sreq.Header.Set("Content-Type", "application/json")
	sreq.RemoteAddr = "127.0.0.1:1234"
	sw := httptest.NewRecorder()
	SessionHandler(svcCtx)(sw, sreq)

	var sessionResp types.SessionResp
	_ = json.Unmarshal(sw.Body.Bytes(), &sessionResp)

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	betBody, _ := json.Marshal(types.BetReq{
		MerchantId:   "m001",
		UserId:       10001,
		SessionToken: sessionResp.SessionToken,
		GameCode:     "fishing",
		Action:       "cast",
		BetAmount:    100000,
		RoundId:      "bad-envelope-round",
		SequenceId:   sessionResp.NextSequenceId,
	})
	breq := httptest.NewRequest(http.MethodPost, "/api/v1/game/bet", bytes.NewReader(betBody))
	breq.Header.Set("Content-Type", "application/json")
	breq.Header.Set(security.HeaderTimestamp, ts)
	breq.Header.Set(security.HeaderSessionSign, "invalid")
	breq.RemoteAddr = "127.0.0.1:1234"
	bw := httptest.NewRecorder()
	BetHandler(svcCtx)(bw, breq)
	if bw.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", bw.Code, bw.Body.String())
	}
}
