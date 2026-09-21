package idempotent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrTokenReused    = errors.New("idempotency token reused across different keys")
	ErrTokenMissing   = errors.New("missing idempotency token")
	ErrAlreadyClaimed = errors.New("request already in progress")
	ErrResultNotFound = errors.New("idempotent result not found")
)

type Store struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

func NewStore(client *redis.Client, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour // 默认注单幂等结果保存 24 小时
	}
	return &Store{
		client: client,
		ttl:    ttl,
		prefix: "idempotent",
	}
}

func (s *Store) claimKey(merchantCode, key string) string {
	return fmt.Sprintf("%s:claim:%s:%s", s.prefix, merchantCode, key)
}

func (s *Store) resultKey(merchantCode, key string) string {
	return fmt.Sprintf("%s:result:%s:%s", s.prefix, merchantCode, key)
}

func (s *Store) tokenKey(merchantCode, token string) string {
	return fmt.Sprintf("%s:token:%s:%s", s.prefix, merchantCode, token)
}

// GetResult 查询已完成结算的幂等结果
func (s *Store) GetResult(ctx context.Context, merchantCode, key string, dest any) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	raw, err := s.client.Get(ctx, s.resultKey(merchantCode, key)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), dest)
}

// SaveResult 存储终态结果，原子更新并解除 Claim 锁定标记
func (s *Store) SaveResult(ctx context.Context, merchantCode, key string, result any) error {
	if s.client == nil {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	// 保存最终业务执行结果
	pipe.Set(ctx, s.resultKey(merchantCode, key), raw, s.ttl)
	// 将 Claim 标记标记为 DONE
	pipe.Set(ctx, s.claimKey(merchantCode, key), "DONE", s.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

// ReleaseClaim 业务执行失败时显式释放锁，允许后续合法重试
func (s *Store) ReleaseClaim(ctx context.Context, merchantCode, key string) error {
	if s.client == nil {
		return nil
	}
	return s.client.Del(ctx, s.claimKey(merchantCode, key)).Err()
}

// Claim 尝试占有执行权 (支持租户隔离)
func (s *Store) Claim(ctx context.Context, merchantCode, key string) (bool, error) {
	if s.client == nil {
		return true, nil
	}
	// 锁定时间建议不超过 1 分钟，避免未捕获异常导致长久死锁
	return s.client.SetNX(ctx, s.claimKey(merchantCode, key), "PENDING", 60*time.Second).Result()
}

// luaClaimWithToken 原子校验 Token 绑定与占用锁:
// KEYS[1]: tokenKey
// KEYS[2]: claimKey
// ARGV[1]: targetKey (例如 roundID 或 transactionID)
// ARGV[2]: ttlSeconds (锁定时间)
// 返回值:
// 1  -> 成功占有锁 (Token 正确绑定)
// 0  -> 已经被其他请求处理中或已完成
// -1 -> Token 已被别的 targetKey 绑定 (Token 冲突复用)
var luaClaimWithToken = redis.NewScript(`
local tokenKey = KEYS[1]
local claimKey = KEYS[2]
local targetKey = ARGV[1]
local ttl = tonumber(ARGV[2])

local existing = redis.call('GET', tokenKey)
if existing and existing ~= targetKey then
    return -1
end

-- 尝试抢占主键锁
local ok = redis.call('SET', claimKey, 'PENDING', 'EX', ttl, 'NX')
if ok then
    -- 首次锁定，将 token 绑定到 targetKey
    redis.call('SET', tokenKey, targetKey, 'EX', ttl)
    return 1
end

return 0
`)

// ClaimWithToken 原子化将幂等 Token 绑定到目标 Key 并加锁
func (s *Store) ClaimWithToken(ctx context.Context, merchantCode, key, token string) (bool, error) {
	if token == "" {
		return false, ErrTokenMissing
	}
	if s.client == nil {
		return true, nil
	}

	tokenK := s.tokenKey(merchantCode, token)
	claimK := s.claimKey(merchantCode, key)

	res, err := luaClaimWithToken.Run(ctx, s.client, []string{tokenK, claimK}, key, int(s.ttl.Seconds())).Int()
	if err != nil {
		return false, err
	}
	if res == -1 {
		return false, ErrTokenReused
	}
	return res == 1, nil
}

// WaitAndGetResult 针对紧密连击/并发重复请求：如果第一笔请求还在处理中，短暂自旋等待其完成并取回结果
func (s *Store) WaitAndGetResult(ctx context.Context, merchantCode, key string, dest any, maxWait time.Duration) (bool, error) {
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		ok, err := s.GetResult(ctx, merchantCode, key, dest)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return false, nil
			}
		}
	}
}
