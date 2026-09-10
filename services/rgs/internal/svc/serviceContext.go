package svc

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/gameconfig"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/ratelimit"
	"fastgame/pkg/security"
	"fastgame/pkg/session"
	"fastgame/pkg/trace"
	"fastgame/pkg/money"
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
	GameConfig  *gameconfig.Loader
	Kafka       *kafka.Producer
	Guard       *security.Guard
	RateLimit   *ratelimit.Gateway
	Session     *session.Store
	PendingOps  model.WalletPendingOpsModel
	PendingTx   model.PendingTransactionsModel
	ReplayStore model.GameRoundReplayModel
	Trace       *trace.CHRecorder
}

func NewServiceContext(c config.Config) *ServiceContext {
	rdb := goredis.NewClient(&goredis.Options{Addr: c.Redis.Addr})
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	merchants := model.NewMerchantsModel(conn)

	zeroRedis := zeroredis.MustNewRedis(zeroredis.RedisConf{Host: c.Redis.Addr, Type: "node"})

	svcCtx := &ServiceContext{
		Config:     c,
		Redis:      rdb,
		Lock:       lock.NewRedisLock(rdb),
		Idempotent: idempotent.NewStore(rdb, 24*time.Hour),
		Wallet:     newWalletClient(c.Wallet),
		GameConfig: gameconfig.NewLoader(
			merchants,
			model.NewGameConfigsModel(conn),
		),
		Kafka: kafka.NewProducer(c.Kafka.Brokers),
		Guard: security.NewGuard(security.Config{
			SkipSignVerify:   c.Security.SkipSignVerify,
			TimestampWindow:  c.Security.TimestampWindow(),
			MaxClockSkew:     c.Security.MaxClockSkew(),
			MinResponseDelay: c.Security.MinResponseDelay(),
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
	}

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

func newWalletClient(cfg config.WalletConf) wallet.Client {
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
	})
}
