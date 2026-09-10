package svc

import (
	"time"

	"fastgame/pkg/idempotent"
	"fastgame/pkg/kafka"
	"fastgame/pkg/lock"
	"fastgame/pkg/prng"
	"fastgame/pkg/wallet"
	"fastgame/services/rgs/internal/config"

	"github.com/redis/go-redis/v9"
)

type ServiceContext struct {
	Config     config.Config
	Redis      *redis.Client
	Lock       *lock.RedisLock
	Idempotent *idempotent.Store
	Wallet     wallet.Client
	PRNG       *prng.Engine
	Kafka      *kafka.Producer
}

func NewServiceContext(c config.Config) *ServiceContext {
	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})

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
		PRNG:       prng.NewEngine(c.Game.DefaultRtpTier),
		Kafka:      kafka.NewProducer(c.Kafka.Brokers),
	}
}
