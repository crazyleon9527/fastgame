package outbox

import (
	"crypto/rand"
	"math/big"
	"time"
)

// Backoff 投递失败的退避策略
type Backoff struct {
	tiers []time.Duration
}

// DefaultBackoff 七档退避
func DefaultBackoff() Backoff {
	return Backoff{tiers: []time.Duration{
		1 * time.Second,
		5 * time.Second,
		30 * time.Second,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		6 * time.Hour,
	}}
}

// NewBackoff 自定义档位
func NewBackoff(tiers ...time.Duration) Backoff {
	if len(tiers) == 0 {
		return DefaultBackoff()
	}
	return Backoff{tiers: tiers}
}

// MaxRetries 达到该次数后置为 FAILED 并告警
func (b Backoff) MaxRetries() int {
	if len(b.tiers) == 0 {
		return 1
	}
	return len(b.tiers)
}

// Delay 第 attempt 次失败后的等待时长（带 ±20% 安全抖动）
func (b Backoff) Delay(attempt int) time.Duration {
	if len(b.tiers) == 0 {
		return time.Second
	}
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(b.tiers) {
		idx = len(b.tiers) - 1
	}
	base := b.tiers[idx]

	// 并发安全的加密级抖动计算，消除 math/rand 全局锁竞争
	jitterRange := int64(base) / 5
	if jitterRange <= 0 {
		jitterRange = 1
	}
	n, err := rand.Int(rand.Reader, big.NewInt(jitterRange))
	var jitter int64
	if err == nil {
		jitter = n.Int64()
	}

	return base - time.Duration(int64(base)/10) + time.Duration(jitter)
}
