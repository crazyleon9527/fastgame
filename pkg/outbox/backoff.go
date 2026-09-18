package outbox

import (
	"math/rand"
	"time"
)

// Backoff 投递失败的退避策略。
//
// 采用固定档位而非纯指数：早期快速重试（1s/5s/30s）能兜住 Kafka 短暂抖动，
// 后期拉长（2h/6h）避免无意义地反复打日志与打 Kafka。
// 每档叠加 ±20% 抖动，防止多实例在同一时刻齐步重试（thundering herd）。
type Backoff struct {
	tiers []time.Duration
}

// DefaultBackoff 七档退避，与 platform-api 的档位一致。
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

// NewBackoff 自定义档位（测试用）。
func NewBackoff(tiers ...time.Duration) Backoff {
	if len(tiers) == 0 {
		return DefaultBackoff()
	}
	return Backoff{tiers: tiers}
}

// MaxRetries 达到该次数后置为 FAILED 并告警，不再自动重试。
func (b Backoff) MaxRetries() int {
	if len(b.tiers) == 0 {
		return 1
	}
	return len(b.tiers)
}

// Delay 第 attempt 次失败后的等待时长（attempt 从 1 开始）。
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
	// ±20% 抖动
	jitter := time.Duration(rand.Int63n(int64(base)/5 + 1))
	return base - time.Duration(int64(base)/10) + jitter
}
