package security

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type ReplayGuard struct {
	client *redis.Client
	window time.Duration
	ttl    time.Duration
}

func NewReplayGuard(client *redis.Client, window, ttl time.Duration) *ReplayGuard {
	if window <= 0 {
		window = 60 * time.Second
	}
	if window < 30*time.Second {
		window = 30 * time.Second
	}
	if window > 60*time.Second {
		window = 60 * time.Second
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ReplayGuard{client: client, window: window, ttl: ttl}
}

func (g *ReplayGuard) Validate(ctx context.Context, timestamp, nonce string) error {
	if timestamp == "" || nonce == "" {
		return fmt.Errorf("missing timestamp or nonce")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}

	now := time.Now().Unix()
	if ts < now-int64(g.window.Seconds()) || ts > now+30 {
		return fmt.Errorf("timestamp expired")
	}

	key := fmt.Sprintf("security:nonce:%s", nonce)
	ok, err := g.client.SetNX(ctx, key, "1", g.ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nonce reused")
	}
	return nil
}
