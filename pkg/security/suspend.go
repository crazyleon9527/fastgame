package security

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type SuspendStore struct {
	client *redis.Client
}

func NewSuspendStore(client *redis.Client) *SuspendStore {
	return &SuspendStore{client: client}
}

func (s *SuspendStore) IsUserSuspended(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	n, err := s.client.Exists(ctx, fmt.Sprintf("rtp:suspend:user:%d", userID)).Result()
	return n > 0, err
}

func (s *SuspendStore) IsGameFlagged(ctx context.Context, merchantCode, gameCode string) (bool, error) {
	key := fmt.Sprintf("rtp:flag:game:%s:%s", merchantCode, gameCode)
	n, err := s.client.Exists(ctx, key).Result()
	return n > 0, err
}
