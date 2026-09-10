package svc

import (
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/gameconfig"
	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/wallet"
	"fastgame/services/rgs/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config     config.Config
	Redis      *redis.Client
	Lock       *lock.RedisLock
	Idempotent *idempotent.Store
	Wallet     wallet.Client
	GameConfig *gameconfig.Loader
	Kafka      *kafka.Producer
}

func NewServiceContext(c config.Config) *ServiceContext {
	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	return &ServiceContext{
		Config:     c,
		Redis:      rdb,
		Lock:       lock.NewRedisLock(rdb),
		Idempotent: idempotent.NewStore(rdb, 24*time.Hour),
		Wallet:     newWalletClient(c.Wallet),
		GameConfig: gameconfig.NewLoader(
			model.NewMerchantsModel(conn),
			model.NewGameConfigsModel(conn),
		),
		Kafka: kafka.NewProducer(c.Kafka.Brokers),
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
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Timeout: timeout,
	})
}
