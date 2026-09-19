package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"fastgame/pkg/ledger"
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
	return newIntegrationSvcCtxWithWallet(t, prodSecurity, nil)
}

// newIntegrationSvcCtxWithWallet 允许注入自定义钱包客户端。
// 传 nil 用默认的 mock 钱包；需要模拟"钱包失败"的场景时传一个会报错的实现。
func newIntegrationSvcCtxWithWallet(t *testing.T, prodSecurity bool, w wallet.Client) *svc.ServiceContext {
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
	if w == nil {
		w = wallet.NewMockClient(money.FromMajor(10000))
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
		Wallet:     w,
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
		// 账变收口也走真实库：下注与派彩都会写 game_transactions，
		// 于是集成测试顺带覆盖账变链路（迁移 35/36 未应用时会直接失败）。
		Ledger: ledger.NewPoster(
			sqlx.NewMysql(integrationDSN()),
			ledger.NewOutboxSink(outbox.NewStore(sqlx.NewMysql(integrationDSN())), "rgs-api-test"),
		),
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

// failingWinWallet 只在 Win 上失败，其余委托给 mock 钱包。
// 用来验证"派彩失败 → 账变记 PENDING_RETRY 且不动余额"这条接线：
// 这是最容易悄悄断掉的一环（记账被删掉不会有任何报错，只是补偿对账时才发现少了一半）。
type failingWinWallet struct {
	inner *wallet.MockClient
}

func (w *failingWinWallet) GetBalance(ctx context.Context, merchantID, userID string) (money.Amount, error) {
	return w.inner.GetBalance(ctx, merchantID, userID)
}

func (w *failingWinWallet) Bet(ctx context.Context, req wallet.BetReq) (*wallet.Result, error) {
	return w.inner.Bet(ctx, req)
}

func (w *failingWinWallet) Win(context.Context, wallet.WinReq) (*wallet.Result, error) {
	return nil, errors.New("探针：模拟钱包派彩超时")
}

func (w *failingWinWallet) Rollback(ctx context.Context, req wallet.RollbackReq) error {
	return w.inner.Rollback(ctx, req)
}

func (w *failingWinWallet) CheckTransaction(ctx context.Context, merchantID, userID, roundID string) (*wallet.TxCheckResult, error) {
	return w.inner.CheckTransaction(ctx, merchantID, userID, roundID)
}

// TestBetWinFailureRecordsPendingRetryLedger —— 派彩失败时账本必须留痕且不动余额。
//
// 断言的是三件事：
//  1. 响应是 202 + settlementStatus=pending（下注本身成功、结算待补偿）；
//  2. game_transactions 里有 (merchant, round, WIN, PENDING_RETRY) 这一条，
//     金额等于该局应派彩额；
//  3. 这条流水的 balance_before == balance_after —— 钱到底动没动还没确认，
//     绝不能改本地余额镜像。
//
// 首次派彩尝试是 PENDING_RETRY、补偿成功后再补一条 SUCCESS，
// 这正是账变幂等键必须带 status 的原因。
func TestBetWinFailureRecordsPendingRetryLedger(t *testing.T) {
	db, err := sql.Open("mysql", integrationDSN())
	if err != nil {
		t.Skipf("跳过：%v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("跳过：MySQL 不可达（%v）", err)
	}

	// 注意注册顺序与 t.Cleanup 的 LIFO 语义：
	// 关闭连接必须**最后**执行，否则清理语句会落在已关闭的连接上静默失败，
	// 每跑一次测试就在开发库里留一批探针行（这个坑踩过一次）。
	t.Cleanup(func() { db.Close() })

	const prefix = "zz-pending-retry-"
	t.Cleanup(func() {
		db.Exec("DELETE FROM game_transactions WHERE round_id LIKE ?", prefix+"%")
		db.Exec("DELETE FROM game_round_replay WHERE round_id LIKE ?", prefix+"%")
		db.Exec("DELETE FROM pending_transactions WHERE round_id LIKE ?", prefix+"%")
		db.Exec("DELETE FROM wallet_pending_ops WHERE round_id LIKE ?", prefix+"%")
	})
	db.Exec("DELETE FROM game_transactions WHERE round_id LIKE ?", prefix+"%")
	db.Exec("DELETE FROM game_round_replay WHERE round_id LIKE ?", prefix+"%")
	db.Exec("DELETE FROM pending_transactions WHERE round_id LIKE ?", prefix+"%")
	db.Exec("DELETE FROM wallet_pending_ops WHERE round_id LIKE ?", prefix+"%")

	svcCtx := newIntegrationSvcCtxWithWallet(t, false, &failingWinWallet{inner: wallet.NewMockClient(money.FromMajor(10000))})

	sessionBody, _ := json.Marshal(types.SessionReq{
		MerchantId: "m001", UserId: "10001", GameCode: "fishing", ClientSeed: "pending-retry-seed",
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

	// 派彩是否发生由 PAR 表决定（约 43% 的局有派彩），因此多试几局直到遇到一次派彩。
	// 40 局全空杆的概率约 0.57^40 ≈ 4e-10，真出现说明赔付表被改坏了。
	var hitRound string
	var wantWin int64
	for i := 0; i < 40; i++ {
		roundID := fmt.Sprintf("%s%d", prefix, i)
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
			SequenceId:   sessionResp.NextSequenceId + uint64(i),
		})
		breq := httptest.NewRequest(http.MethodPost, "/api/v1/game/bet", bytes.NewReader(betBody))
		breq.Header.Set("Content-Type", "application/json")
		// 该测试配置只跳过商户签名，会话信封签名仍要校验
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
		if betResp.SettlementStatus == "pending" {
			if bw.Code != http.StatusAccepted {
				t.Errorf("结算待补偿时应当返回 202，实际 %d", bw.Code)
			}
			hitRound, wantWin = roundID, betResp.WinAmount
			break
		}
	}
	if hitRound == "" {
		t.Fatal("40 局都没遇到派彩，无法验证 PENDING_RETRY 接线（赔付表可能被改坏）")
	}
	if wantWin <= 0 {
		t.Fatalf("待补偿局的派彩额 = %d，应大于 0", wantWin)
	}

	var (
		gotStatus string
		gotAmount int64
		before    int64
		after     int64
	)
	err = db.QueryRow(
		`SELECT status, amount, balance_before, balance_after FROM game_transactions
		 WHERE merchant_id = ? AND round_id = ? AND tx_type = 'WIN'`, stubMerchantID, hitRound).
		Scan(&gotStatus, &gotAmount, &before, &after)
	if err != nil {
		t.Fatalf("派彩失败后没有留下账变流水（接线断了）: %v", err)
	}
	if gotStatus != "PENDING_RETRY" {
		t.Errorf("流水状态 = %s，期望 PENDING_RETRY", gotStatus)
	}
	if gotAmount != wantWin {
		t.Errorf("流水金额 = %d，期望 %d", gotAmount, wantWin)
	}
	if before != after {
		t.Errorf("待补偿流水不应改动余额镜像: before=%d after=%d", before, after)
	}
}

// assertReplayMerchantID 校验回放记录确实落在期望商户下：
// (merchant_id, round_id) 这一行必须存在。若结算收尾没把商户写进唯一键前导列，
// MySQL 不可达时跳过，避免把单元测试变成环境依赖。
func assertReplayMerchantID(t *testing.T, roundID string, want uint64) {
	t.Helper()
	db, err := sql.Open("mysql", integrationDSN())
	if err != nil {
		t.Skipf("跳过 merchant_id 校验：%v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("跳过 merchant_id 校验：MySQL 不可达（%v）", err)
	}
	// t.Cleanup 是 LIFO：先注册关闭，保证清理语句执行时连接仍然可用
	t.Cleanup(func() { db.Close() })

	// 本测试新建的行自己清掉，避免污染开发库
	t.Cleanup(func() {
		db.Exec("DELETE FROM game_round_replay WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM pending_transactions WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM wallet_pending_ops WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM game_transactions WHERE round_id = ?", roundID)
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
