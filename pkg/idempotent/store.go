package idempotent

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
	ttl    time.Duration
}

func NewStore(client *redis.Client, ttl time.Duration) *Store {
	return &Store{client: client, ttl: ttl}
}

func (s *Store) key(roundID string) string {
	return fmt.Sprintf("idempotent:round:%s", roundID)
}

// Claim returns false if the round was already processed.
func (s *Store) Claim(ctx context.Context, roundID string) (bool, error) {
	return s.client.SetNX(ctx, s.key(roundID), "1", s.ttl).Result()
}
