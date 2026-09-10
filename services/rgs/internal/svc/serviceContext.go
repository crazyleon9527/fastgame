package svc

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/gameconfig"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/security"
	"fastgame/pkg/session"
	"fastgame/pkg/wallet"
	"fastgame/services/rgs/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config      config.Config
	Redis       *redis.Client
	Lock        *lock.RedisLock
	Idempotent  *idempotent.Store
	Wallet      wallet.Client
	GameConfig  *gameconfig.Loader
	Kafka       *kafka.Producer
	Guard       *security.Guard
	Session     *session.Store
	PendingOps  model.WalletPendingOpsModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	merchants := model.NewMerchantsModel(conn)

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
			SkipSignVerify:     c.Security.SkipSignVerify,
			TimestampWindow:    c.Security.TimestampWindow(),
			UserBetLimitPerMin: c.Security.UserBetLimitPerMin,
			IPLimitPerSec:      c.Security.IPLimitPerSec,
			MinResponseDelay:   c.Security.MinResponseDelay(),
		}, merchants, rdb),
		Session:    session.NewStore(rdb, c.Session.TTL()),
		PendingOps: model.NewWalletPendingOpsModel(conn),
	}

	bootstrapBlacklist(rdb, model.NewRiskBlacklistModel(conn))
	return svcCtx
}

func bootstrapBlacklist(rdb *redis.Client, blacklistModel model.RiskBlacklistModel) {
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
	if cfg.Mock || cfg.BaseURL == "" {
		return wallet.NewMockClient(10000)
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		timeout = 5 * time.Second
	}
	return wallet.NewHTTPClient(wallet.HTTPConfig{
		BaseURL:              cfg.BaseURL,
		APIKey:               cfg.APIKey,
		Secret:               cfg.SignSecret,
		SignEnabled:          cfg.SignEnabled,
		VerifyResponse:       cfg.VerifyResponse,
		ResponseTimestampWin: cfg.ResponseTimestampWindow(),
		Timeout:              timeout,
	})
}
