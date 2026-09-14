package svc

import (
	"fastgame/engine"
	_ "fastgame/engine/games/fishing" // 触发 FishingPlugin 自动注册
	"fastgame/pkg/lock"
	"fastgame/pkg/wallet"
	"fastgame/service/rgs/api/internal/config"

	"github.com/redis/go-redis/v9"
)

type ServiceContext struct {
	Config config.Config
	Host   *engine.UniversalHost
	Redis  *redis.Client
	Lock   *lock.RedisLock
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 1. 初始化原生 redis/v9 客户端 (与 pkg/lock 完全兼容)
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Host,
		Password: c.Redis.Pass,
		DB:       0,
	})

	// 2. 初始化 RedisLock 分布式锁
	redisLock := lock.NewRedisLock(rdb)

	// 3. 初始化 Seamless 钱包 Client (对接本地 Mock 钱包 8089 端口或远端商户)
	walletClient := wallet.NewHTTPClient(wallet.HTTPConfig{
		BaseURL:     c.Wallet.BaseURL, // http://127.0.0.1:8089
		APIKey:      c.Wallet.APIKey,
		Secret:      c.Wallet.Secret,
		SignEnabled: c.Wallet.SignEnabled,
		Timeout:     c.Wallet.Timeout,
	})

	// 4. 组装 UniversalHost 核心调度管道
	host := engine.NewUniversalHost(
		walletClient,
		redisLock,
		nil, // dlqWriter: 本地联调传 nil，后续接 MySQL DLQ
		nil, // producer: 本地联调传 nil，后续接 Kafka
		"game_round_settled",
	)

	return &ServiceContext{
		Config: c,
		Host:   host,
		Redis:  rdb,
		Lock:   redisLock,
	}
}
