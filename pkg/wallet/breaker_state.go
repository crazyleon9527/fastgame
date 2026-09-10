package wallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var breakerOpenGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "fastgame",
		Subsystem: "wallet",
		Name:      "breaker_open",
		Help:      "1 when wallet circuit breaker is open for a merchant",
	},
	[]string{"merchant"},
)

func init() {
	prometheus.MustRegister(breakerOpenGauge)
}

// BreakerStatus 当前熔断状态
type BreakerStatus struct {
	MerchantCode string `json:"merchantCode"`
	Open         bool   `json:"open"`
	OpenedAt     int64  `json:"openedAt,omitempty"`
	Overridden   bool   `json:"overridden"`
}

func SetBreakerMetric(merchantID string, open bool) {
	v := 0.0
	if open {
		v = 1
	}
	breakerOpenGauge.WithLabelValues(merchantID).Set(v)
}

// ListOpenBreakers 扫描 Redis 中 wallet:breaker:open:* 与 override 状态
func ListOpenBreakers(ctx context.Context, rdb *redis.Client) ([]BreakerStatus, error) {
	if rdb == nil {
		return nil, nil
	}
	var out []BreakerStatus
	iter := rdb.Scan(ctx, 0, "wallet:breaker:open:*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		merchant := strings.TrimPrefix(key, "wallet:breaker:open:")
		if merchant == "" {
			continue
		}
		ts, _ := rdb.Get(ctx, key).Int64()
		overridden, _ := rdb.Exists(ctx, fmt.Sprintf("wallet:breaker:override:%s", merchant)).Result()
		out = append(out, BreakerStatus{
			MerchantCode: merchant,
			Open:         true,
			OpenedAt:     ts,
			Overridden:   overridden > 0,
		})
		SetBreakerMetric(merchant, true)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func breakerOpenTTL() time.Duration {
	return 30 * time.Minute
}
