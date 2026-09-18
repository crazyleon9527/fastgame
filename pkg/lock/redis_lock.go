package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrBusy     = errors.New("lock busy")
	ErrLockHeld = errors.New("lock not held or token mismatch")
)

const DefaultBetLockTTL = 3 * time.Second

// BetLockKey 多租户商户与玩家隔离的锁 Key
func BetLockKey(merchantCode, userID string) string {
	return fmt.Sprintf("lock:bet:%s:%s", merchantCode, userID)
}

const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`

type RedisLock struct {
	client *redis.Client
	mu     sync.Mutex
	tokens map[string]string // 记录当前实例已成功获取的 key -> token
}

func NewRedisLock(client *redis.Client) *RedisLock {
	return &RedisLock{
		client: client,
		tokens: make(map[string]string),
	}
}

// Acquire 满足 engine.RedisLock 接口：原子加锁
func (l *RedisLock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = DefaultBetLockTTL
	}

	token, err := generateSecureToken()
	if err != nil {
		return false, fmt.Errorf("generate lock token failed: %w", err)
	}

	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	l.mu.Lock()
	l.tokens[key] = token
	l.mu.Unlock()

	return true, nil
}

// Release 满足 engine.RedisLock 接口：基于 Lua 脚本安全释放
func (l *RedisLock) Release(ctx context.Context, key string) error {
	l.mu.Lock()
	token, ok := l.tokens[key]
	delete(l.tokens, key)
	l.mu.Unlock()

	if !ok {
		return ErrLockHeld
	}

	// 使用独立的 1 秒超时 context，防止业务 ctx 已超时导致锁无法释放
	releaseCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	res, err := l.client.Eval(releaseCtx, unlockScript, []string{key}, token).Result()
	if err != nil {
		return fmt.Errorf("release lock failed: %w", err)
	}
	if n, _ := res.(int64); n == 0 {
		return ErrLockHeld
	}

	return nil
}

// WithLock 经典闭包式调用
func (l *RedisLock) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	acquired, err := l.Acquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrBusy
	}

	defer func() {
		_ = l.Release(ctx, key)
	}()

	return fn()
}

// generateSecureToken 生成机器纳秒 + 8 字节密码学随机串，避免 token 碰撞
func generateSecureToken() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), hex.EncodeToString(b)), nil
}
