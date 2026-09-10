package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	client *redis.Client
}

func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{client: client}
}

// Allow checks sliding-window counter. Returns false when quota exceeded.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	now := time.Now().UnixMilli()
	windowStart := now - window.Milliseconds()
	redisKey := fmt.Sprintf("ratelimit:%s", key)

	pipe := l.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: fmt.Sprintf("%d", now)})
	pipe.ZCard(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window+time.Second)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count, err := cmds[2].(*redis.IntCmd).Result()
	if err != nil {
		return false, err
	}
	return count <= int64(limit), nil
}
