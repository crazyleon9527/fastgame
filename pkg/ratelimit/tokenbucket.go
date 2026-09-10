package ratelimit

import (
	"context"
	"fmt"
	"sync"

	"github.com/zeromicro/go-zero/core/limit"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Gateway wraps go-zero TokenLimiter (token bucket) for per-IP and per-user QPS.
type Gateway struct {
	store     *redis.Redis
	ipRate    int
	ipBurst   int
	userRate  int
	userBurst int
	ipPool    sync.Map
	userPool  sync.Map
}

func NewGateway(store *redis.Redis, ipRate, ipBurst, userRate, userBurst int) *Gateway {
	if ipRate <= 0 {
		ipRate = 20
	}
	if ipBurst <= 0 {
		ipBurst = ipRate
	}
	if userRate <= 0 {
		userRate = 5
	}
	if userBurst <= 0 {
		userBurst = userRate
	}
	return &Gateway{
		store:     store,
		ipRate:    ipRate,
		ipBurst:   ipBurst,
		userRate:  userRate,
		userBurst: userBurst,
	}
}

func (g *Gateway) AllowIP(ctx context.Context, ip string) bool {
	if ip == "" {
		return true
	}
	key := fmt.Sprintf("ip:%s", ip)
	limiter := g.loadLimiter(&g.ipPool, key, g.ipRate, g.ipBurst)
	return limiter.AllowCtx(ctx)
}

func (g *Gateway) AllowUser(ctx context.Context, userID uint64) bool {
	if userID == 0 {
		return true
	}
	key := fmt.Sprintf("user:%d", userID)
	limiter := g.loadLimiter(&g.userPool, key, g.userRate, g.userBurst)
	return limiter.AllowCtx(ctx)
}

func (g *Gateway) loadLimiter(pool *sync.Map, key string, rate, burst int) *limit.TokenLimiter {
	if val, ok := pool.Load(key); ok {
		return val.(*limit.TokenLimiter)
	}
	limiter := limit.NewTokenLimiter(rate, burst, g.store, key)
	val, _ := pool.LoadOrStore(key, limiter)
	return val.(*limit.TokenLimiter)
}
