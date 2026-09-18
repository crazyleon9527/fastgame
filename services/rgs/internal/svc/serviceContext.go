package svc

import (
	"context"
	"fmt"
	"os"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/fieldcipher"
	"fastgame/pkg/gameconfig"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/money"
	"fastgame/pkg/outbox"
	"fastgame/pkg/ratelimit"
	"fastgame/pkg/security"
	"fastgame/pkg/session"
	"fastgame/pkg/trace"
	"fastgame/pkg/wallet"
	"fastgame/services/rgs/internal/config"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	zeroredis "github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config      config.Config
	Redis       *goredis.Client
	Lock        *lock.RedisLock
	Idempotent  *idempotent.Store
	Wallet      wallet.Client
	GameConfig  gameconfig.Provider
	Kafka       *kafka.Producer
	Guard       *security.Guard
	RateLimit   *ratelimit.Gateway
	Session     *session.Store
	PendingOps  model.WalletPendingOpsModel
	PendingTx   model.PendingTransactionsModel
	ReplayStore model.GameRoundReplayModel
	Trace       *trace.CHRecorder

	// DB 供需要事务的地方使用（例如把结算事件与业务写放进同一事务）
	DB               sqlx.SqlConn
	Outbox           *outbox.Store
	OutboxDispatcher *outbox.Dispatcher
}

func NewServiceContext(c config.Config) *ServiceContext {
	fieldcipher.Init(c.Security.MerchantKeyCipher)

	rdb := goredis.NewClient(&goredis.Options{Addr: c.Redis.Addr})
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	merchants := model.NewMerchantsModel(conn)

	zeroRedis := zeroredis.MustNewRedis(zeroredis.RedisConf{Host: c.Redis.Addr, Type: "node"})

	svcCtx := &ServiceContext{
		Config:     c,
		Redis:      rdb,
		Lock:       lock.NewRedisLock(rdb),
		Idempotent: idempotent.NewStore(rdb, 24*time.Hour),
		Wallet:     newWalletClient(c.Wallet, rdb),
		GameConfig: gameconfig.NewLoader(
			merchants,
			model.NewGameConfigsModel(conn),
		),
		Kafka: kafka.NewProducer(c.Kafka.Brokers),
		Guard: security.NewGuard(security.Config{
			SkipSignVerify:      c.Security.SkipSignVerify,
			SkipMerchantSign:    c.Security.SkipMerchantSign,
			SkipSessionEnvelope: c.Security.SkipSessionEnvelope,
			TimestampWindow:     c.Security.TimestampWindow(),
			MaxClockSkew:        c.Security.MaxClockSkew(),
			MinResponseDelay:    c.Security.MinResponseDelay(),
		}, merchants, rdb),
		RateLimit: ratelimit.NewGateway(
			zeroRedis,
			c.Security.IPLimitPerSec, c.Security.IPLimitPerSec,
			c.Security.UserLimitPerSec, c.Security.UserLimitPerSec,
		),
		Session:    session.NewStore(rdb, c.Session.TTL()),
		PendingOps:  model.NewWalletPendingOpsModel(conn),
		PendingTx:   model.NewPendingTransactionsModel(conn),
		ReplayStore: model.NewGameRoundReplayModel(conn),
		// 事务性发件箱：结算事件不再裸 go func 投递 Kafka，而是与业务写同事务落表，
		// 由 OutboxDispatcher 负责可靠投递（投递失败退避重试）。
		DB:     conn,
		Outbox: outbox.NewStore(conn),
	}

	// 派发器依赖已构造好的 svcCtx（需要其中的 Kafka producer），故在其之后装配。
	svcCtx.OutboxDispatcher = newOutboxDispatcher(conn, svcCtx)

	if c.CH.Addr != "" {
		writer, err := clickhouse.NewWriterWithAuth(c.CH.Addr, c.CH.Database, c.CH.User, c.CH.Password)
		if err != nil {
			logx.Errorf("clickhouse trace writer disabled: %v", err)
		} else {
			svcCtx.Trace = trace.NewCHRecorder(writer)
		}
	}

	bootstrapBlacklist(rdb, model.NewRiskBlacklistModel(conn))
	return svcCtx
}

// newOutboxDispatcher 组装派发器。owner 用 "主机名:pid"，
// SettleBatch 靠它 + status 双守卫，避免被 Reaper 回收后旧 owner 误标状态。
func newOutboxDispatcher(conn sqlx.SqlConn, svcCtx *ServiceContext) *outbox.Dispatcher {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return outbox.NewDispatcher(
		conn,
		outbox.NewKafkaDeliverer(svcCtx.Kafka),
		outbox.DispatcherConfig{
			Owner:     fmt.Sprintf("rgs-api:%s:%d", host, os.Getpid()),
			BatchSize: 100,
		},
	)
}

func bootstrapBlacklist(rdb *goredis.Client, blacklistModel model.RiskBlacklistModel) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items, err := blacklistModel.ListAllActive(ctx)
	if err != nil {
		logx.Errorf("bootstrap blacklist failed: %v", err)
		return
	}
	if err := security.NewBlacklist(rdb).SyncAll(ctx, items); err != nil {
		logx.Errorf("sync blacklist to redis failed: %v", err)
	}
}

func newWalletClient(cfg config.WalletConf, rdb *goredis.Client) wallet.Client {
	var inner wallet.Client
	if cfg.Mock || cfg.BaseURL == "" {
		inner = wallet.NewMockClient(money.FromMajor(10000))
	} else {
		timeout, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			timeout = 5 * time.Second
		}
		inner = wallet.NewHTTPClient(wallet.HTTPConfig{
			BaseURL:              cfg.BaseURL,
			APIKey:               cfg.APIKey,
			Secret:               cfg.SignSecret,
			SignEnabled:          cfg.SignEnabled,
			VerifyResponse:       cfg.VerifyResponse,
			ResponseTimestampWin: cfg.ResponseTimestampWindow(),
			Timeout:              timeout,
		})
	}
	return wallet.NewBreakerClient(inner, wallet.BreakerConfig{
		SlowThreshold: cfg.SlowThreshold(),
		Redis:         rdb,
	})
}
