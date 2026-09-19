package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/gameconfig"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/money"
	"fastgame/pkg/outbox"
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
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type stubGameConfig struct{}

type noopPendingTx struct{}

func (noopPendingTx) Insert(context.Context, *model.PendingTransaction) error { return nil }
func (noopPendingTx) MarkSettled(context.Context, uint64, string) error       { return nil }
func (noopPendingTx) MarkWinPending(context.Context, uint64, string, int64, string) error {
	return nil
}
func (noopPendingTx) ListStalePending(context.Context, time.Duration, int) ([]*model.PendingTransaction, error) {
	return nil, nil
}
func (noopPendingTx) FindByTraceID(context.Context, string) ([]*model.PendingTransaction, error) {
	return nil, nil
}
func (noopPendingTx) FindByRoundID(context.Context, uint64, string) (*model.PendingTransaction, error) {
	return nil, model.ErrNotFound
}
func (noopPendingTx) MarkDone(context.Context, uint64) error               { return nil }
func (noopPendingTx) MarkFailed(context.Context, uint64, string) error     { return nil }
func (noopPendingTx) IncrementRetry(context.Context, uint64, string) error { return nil }

type noopPendingOps struct{}

func (noopPendingOps) Insert(context.Context, *model.WalletPendingOp) (sql.Result, error) {
	return nil, nil
}
func (noopPendingOps) FindByRoundOp(context.Context, uint64, string, string) (*model.WalletPendingOp, error) {
	return nil, model.ErrNotFound
}
func (noopPendingOps) ListPending(context.Context, int) ([]*model.WalletPendingOp, error) {
	return nil, nil
}
func (noopPendingOps) MarkDone(context.Context, uint64) error               { return nil }
func (noopPendingOps) MarkFailed(context.Context, uint64, string) error     { return nil }
func (noopPendingOps) IncrementRetry(context.Context, uint64, string) error { return nil }

type noopReplayStore struct{}

func (noopReplayStore) Insert(context.Context, *model.GameRoundReplay) error { return nil }
func (noopReplayStore) FindByRoundID(context.Context, uint64, string) (*model.GameRoundReplay, error) {
	return nil, model.ErrNotFound
}

// stubGameConfig 返回商户 m001 的配置。
// MerchantID 必须是 merchants.id（m001 => 1）：对账类表（pending_transactions /
// game_round_replay）的唯一键以 merchant_id 前导，返回 0 会让集成测试跑在
// 「merchant_id=0」这条错误路径上，恰好掩盖真实缺陷。
func (stubGameConfig) Load(_ context.Context, _, _ string) (*gameconfig.Config, error) {
	return &gameconfig.Config{
		MerchantID: stubMerchantID,
		RtpTier:    "default",
		Raw: map[string]json.RawMessage{
			"bet_limits": json.RawMessage(`{"min":10000,"max":10000000,"allowed":[100000]}`),
		},
	}, nil
}

func (stubGameConfig) MerchantID(_ context.Context, merchantCode string) (uint64, error) {
	if merchantCode != "m001" {
		return 0, model.ErrNotFound
	}
	return stubMerchantID, nil
}

const testMerchantSecret = "dev-secret-m001-change-in-prod"

// stubMerchantID 是商户编码 m001 在 merchants 表里的 id
// （与 docker/mysql/init 的种子数据一致：m001 => 1）。
const stubMerchantID uint64 = 1

func newIntegrationSvcCtx(t *testing.T, prodSecurity bool) *svc.ServiceContext {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	_ = rdb.Set(ctx, "merchant:allowips:m001", "[]", time.Hour).Err()

	zeroRedis := zeroredis.MustNewRedis(zeroredis.RedisConf{Host: mr.Addr(), Type: "node"})

	skipMerchant := !prodSecurity
	var merchants model.MerchantsModel
	if prodSecurity {
		merchants = model.StubMerchantsModel{
			Secrets: &model.MerchantSecrets{
				PrivateKey: sql.NullString{String: testMerchantSecret, Valid: true},
			},
		}
	}

	return &svc.ServiceContext{
		Config: config.Config{
			Security: config.SecurityConf{
				SkipMerchantSign:    skipMerchant,
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
			SkipMerchantSign:    skipMerchant,
			SkipSessionEnvelope: false,
			TimestampWindow:     120 * time.Second,
			MaxClockSkew:        5 * time.Second,
		}, merchants, rdb),
		RateLimit:   ratelimit.NewGateway(zeroRedis, 1000, 1000, 1000, 1000),
		Session:     session.NewStore(rdb, time.Hour),
		PendingTx:   noopPendingTx{},
		PendingOps:  noopPendingOps{},
		ReplayStore: noopReplayStore{},
		// 结算收尾（recordSettled）现在把「对账标记 + 回放记录 + 事件登记」
		// 收进同一事务，因此集成测试也需要真实 MySQL；
		// 用与 internal/model 测试一致的 DSN，可用 MYSQL_DSN 覆盖。
		DB:     sqlx.NewMysql(integrationDSN()),
		Outbox: outbox.NewStore(sqlx.NewMysql(integrationDSN())),
	}
}

// integrationDSN 集成测试的 MySQL 连接串（与 docker compose 暴露的端口一致）。
func integrationDSN() string {
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		return v
	}
	return "fastgame:fastgame_pass@tcp(127.0.0.1:13306)/fastgame?charset=utf8mb4&parseTime=true&loc=UTC"
}

func signMerchantRequest(method, path, body, ts, nonce, secret string) string {
	payload := security.BuildSignPayload(method, path, body, ts, nonce)
	return security.Sign(secret, payload)
}

func TestBetGoldenPathSessionEnvelope(t *testing.T) {
	svcCtx := newIntegrationSvcCtx(t, false)

	// 1) Create session
	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001",
		UserId:     "10001",
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
	//
	// round_id 每次运行都不同：唯一键是 (merchant_id, round_id)，用固定 round_id
	// 第二次运行会命中 on duplicate key update 变成空写，就校验不到 merchant_id
	// 到底有没有写进去。
	roundID := fmt.Sprintf("integration-round-%d", time.Now().UnixNano())
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	envSig := security.SignEnvelope(sessionResp.DynamicSessionKey, roundID, "cast", ts)

	betBody, _ := json.Marshal(types.BetReq{
		MerchantId:   "m001",
		UserId:       "10001",
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

	// 3) 记账落地校验：game_round_replay 的唯一键是 (merchant_id, round_id)，
	// merchant_id 必须写真实值。写成 0 时多商户下不同商户的同一 roundId 会挤到
	// 同一个 (0, roundId) 上互相覆盖，唯一键形同虚设。
	assertReplayMerchantID(t, roundID, stubMerchantID)
}

// assertReplayMerchantID 校验回放记录确实落在期望商户下：
// (merchant_id, round_id) 这一行必须存在。若结算收尾没把商户写进唯一键前导列，
// 落库的会是 (0, round_id)，这里就查不到，测试失败。
// MySQL 不可达时跳过，避免把单元测试变成环境依赖。
func assertReplayMerchantID(t *testing.T, roundID string, want uint64) {
	t.Helper()
	db, err := sql.Open("mysql", integrationDSN())
	if err != nil {
		t.Skipf("跳过 merchant_id 校验：%v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("跳过 merchant_id 校验：MySQL 不可达（%v）", err)
	}

	// 本测试新建的行自己清掉，避免污染开发库
	t.Cleanup(func() {
		db.Exec("DELETE FROM game_round_replay WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM pending_transactions WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM event_outbox WHERE payload LIKE CONCAT('%', ?, '%')", roundID)
	})

	var got uint64
	err = db.QueryRow(
		"SELECT merchant_id FROM game_round_replay WHERE merchant_id = ? AND round_id = ?",
		want, roundID,
	).Scan(&got)
	if err == sql.ErrNoRows {
		var fallback uint64
		fallbackErr := db.QueryRow(
			"SELECT merchant_id FROM game_round_replay WHERE round_id = ? ORDER BY id DESC LIMIT 1",
			roundID,
		).Scan(&fallback)
		if fallbackErr != nil {
			t.Fatalf("game_round_replay 里没有 round_id=%s 的记录（%v）——结算收尾没有落回放记录", roundID, fallbackErr)
		}
		t.Fatalf("game_round_replay.merchant_id = %d，期望 %d —— "+
			"结算收尾没有把商户写进唯一键前导列（uk_merchant_round = merchant_id + round_id）",
			fallback, want)
	}
	if err != nil {
		t.Fatalf("读取 game_round_replay 失败: %v", err)
	}
	if got != want {
		t.Fatalf("game_round_replay.merchant_id = %d，期望 %d", got, want)
	}

	// pending_transactions 由 noopPendingTx 接管，这里只校验回放表；
	// pending_transactions 的 merchant_id 由 internal/model 的回归测试覆盖。
}

func TestBetProdSecurityMerchantSignAndEnvelope(t *testing.T) {
	svcCtx := newIntegrationSvcCtx(t, true)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001",
		UserId:     "10001",
		GameCode:   "fishing",
		ClientSeed: "client-prod-e2e",
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

	roundID := "prod-e2e-round-1"
	betBody, _ := json.Marshal(types.BetReq{
		MerchantId:   "m001",
		UserId:       "10001",
		SessionToken: sessionResp.SessionToken,
		GameCode:     "fishing",
		Action:       "cast",
		BetAmount:    100000,
		RoundId:      roundID,
		SequenceId:   sessionResp.NextSequenceId,
	})
	bBody := string(betBody)
	bNonce := "nonce-prod-e2e-2"
	envSig := security.SignEnvelope(sessionResp.DynamicSessionKey, roundID, "cast", ts)
	breq := httptest.NewRequest(http.MethodPost, "/api/v1/game/bet", bytes.NewReader(betBody))
	breq.Header.Set("Content-Type", "application/json")
	breq.Header.Set(security.HeaderTimestamp, ts)
	breq.Header.Set(security.HeaderNonce, bNonce)
	breq.Header.Set(security.HeaderSignature, signMerchantRequest(http.MethodPost, "/api/v1/game/bet", bBody, ts, bNonce, testMerchantSecret))
	breq.Header.Set(security.HeaderSessionSign, envSig)
	breq.RemoteAddr = "127.0.0.1:1234"
	bw := httptest.NewRecorder()
	BetHandler(svcCtx)(bw, breq)
	if bw.Code != http.StatusOK && bw.Code != http.StatusAccepted {
		t.Fatalf("bet status=%d body=%s", bw.Code, bw.Body.String())
	}
}

func TestBetRejectsBadMerchantSign(t *testing.T) {
	svcCtx := newIntegrationSvcCtx(t, true)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001",
		UserId:     "10001",
		GameCode:   "fishing",
	})
	sreq := httptest.NewRequest(http.MethodPost, "/api/v1/game/session", bytes.NewReader(sessionBody))
	sreq.Header.Set("Content-Type", "application/json")
	sreq.RemoteAddr = "127.0.0.1:1234"
	sw := httptest.NewRecorder()
	SessionHandler(svcCtx)(sw, sreq)
	var sessionResp types.SessionResp
	if err := json.Unmarshal(sw.Body.Bytes(), &sessionResp); err != nil {
		t.Fatal(err)
	}

	roundID := "bad-sign-round"
	betBody, _ := json.Marshal(types.BetReq{
		MerchantId:   "m001",
		UserId:       "10001",
		SessionToken: sessionResp.SessionToken,
		GameCode:     "fishing",
		Action:       "cast",
		BetAmount:    100000,
		RoundId:      roundID,
		SequenceId:   sessionResp.NextSequenceId,
	})
	bNonce := "nonce-bad-sign"
	envSig := security.SignEnvelope(sessionResp.DynamicSessionKey, roundID, "cast", ts)
	breq := httptest.NewRequest(http.MethodPost, "/api/v1/game/bet", bytes.NewReader(betBody))
	breq.Header.Set("Content-Type", "application/json")
	breq.Header.Set(security.HeaderTimestamp, ts)
	breq.Header.Set(security.HeaderNonce, bNonce)
	breq.Header.Set(security.HeaderSignature, "deadbeef")
	breq.Header.Set(security.HeaderSessionSign, envSig)
	breq.RemoteAddr = "127.0.0.1:1234"
	bw := httptest.NewRecorder()
	BetHandler(svcCtx)(bw, breq)
	if bw.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad merchant sign, got %d body=%s", bw.Code, bw.Body.String())
	}
}

func TestBetRejectsInvalidEnvelope(t *testing.T) {
	svcCtx := newIntegrationSvcCtx(t, false)

	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001",
		UserId:     "10001",
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
		UserId:       "10001",
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
