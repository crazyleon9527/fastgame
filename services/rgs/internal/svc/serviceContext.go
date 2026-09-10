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

	var walletClient wallet.Client
	if c.Wallet.Mock {
		walletClient = wallet.NewMockClient(10000)
	} else {
		walletClient = wallet.NewMockClient(10000) // TODO: replace with real Seamless Wallet HTTP client
	}

	return &ServiceContext{
		Config:     c,
		Redis:      rdb,
		Lock:       lock.NewRedisLock(rdb),
		Idempotent: idempotent.NewStore(rdb, 24*time.Hour),
		Wallet:     walletClient,
		GameConfig: gameconfig.NewLoader(
			model.NewMerchantsModel(conn),
			model.NewGameConfigsModel(conn),
		),
		Kafka: kafka.NewProducer(c.Kafka.Brokers),
	}
}
