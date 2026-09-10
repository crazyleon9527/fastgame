package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrBusy = errors.New("lock busy")

const BetLockTTL = 3 * time.Second

func BetLockKey(userID uint64) string {
	return fmt.Sprintf("lock:bet:%d", userID)
}

const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`

type RedisLock struct {
	client *redis.Client
}

func NewRedisLock(client *redis.Client) *RedisLock {
	return &RedisLock{client: client}
}

func (l *RedisLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrBusy
	}

	defer func() {
		_ = l.client.Eval(ctx, unlockScript, []string{key}, token).Err()
	}()

	return fn()
}
