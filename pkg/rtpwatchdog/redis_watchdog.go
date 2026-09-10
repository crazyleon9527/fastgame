package rtpwatchdog

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// RedisWatchdog 跨 Consumer 副本共享的滑动窗口（Redis List + LTRIM）
type RedisWatchdog struct {
	rdb  *redis.Client
	cfg  Config
	prefix string
}

func NewRedis(rdb *redis.Client, cfg Config) *RedisWatchdog {
	if cfg.GlobalMax <= 0 {
		cfg = DefaultConfig()
	}
	return &RedisWatchdog{rdb: rdb, cfg: cfg, prefix: "rtp:watch"}
}

func (w *RedisWatchdog) Record(ctx context.Context, in RecordInput) ([]Alert, error) {
	if in.BetMinor <= 0 {
		return nil, nil
	}
	sample := fmt.Sprintf("%d|%d", in.BetMinor, in.WinMinor)
	userKey := fmt.Sprintf("%s:user:%d", w.prefix, in.UserID)
	gameKey := fmt.Sprintf("%s:game:%s:%s", w.prefix, in.MerchantCode, in.GameCode)

	if err := w.pushSample(ctx, userKey, sample, w.cfg.PlayerMax); err != nil {
		return nil, err
	}
	if err := w.pushSample(ctx, gameKey, sample, w.cfg.GlobalMax); err != nil {
		return nil, err
	}

	var alerts []Alert
	if alert, ok, err := w.evalKey(ctx, userKey, "user", fmt.Sprintf("%d", in.UserID), in); err != nil {
		return nil, err
	} else if ok {
		alerts = append(alerts, alert)
	}
	if alert, ok, err := w.evalKey(ctx, gameKey, "game", in.MerchantCode+":"+in.GameCode, in); err != nil {
		return nil, err
	} else if ok {
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func (w *RedisWatchdog) pushSample(ctx context.Context, key, sample string, max int) error {
	pipe := w.rdb.Pipeline()
	pipe.LPush(ctx, key, sample)
	pipe.LTrim(ctx, key, 0, int64(max-1))
	_, err := pipe.Exec(ctx)
	return err
}

func (w *RedisWatchdog) evalKey(ctx context.Context, key, scopeType, scopeValue string, in RecordInput) (Alert, bool, error) {
	items, err := w.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return Alert{}, false, err
	}
	if len(items) < 10 {
		return Alert{}, false, nil
	}
	var totalBet, totalWin int64
	for _, item := range items {
		bet, win, err := parseSample(item)
		if err != nil {
			continue
		}
		totalBet += bet
		totalWin += win
	}
	if totalBet < w.cfg.MinSampleBet {
		return Alert{}, false, nil
	}
	rtpPPM := totalWin * 1_000_000 / totalBet
	if rtpPPM <= w.cfg.ThresholdPPM {
		return Alert{}, false, nil
	}
	return Alert{
		ScopeType:    scopeType,
		ScopeValue:   scopeValue,
		MerchantCode: in.MerchantCode,
		GameCode:     in.GameCode,
		UserID:       in.UserID,
		RtpPPM:       rtpPPM,
		TotalBet:     totalBet,
		TotalWin:     totalWin,
		SampleSize:   len(items),
	}, true, nil
}

func parseSample(s string) (bet, win int64, err error) {
	parts := strings.SplitN(s, "|", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("bad sample")
	}
	bet, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	win, err = strconv.ParseInt(parts[1], 10, 64)
	return bet, win, err
}
