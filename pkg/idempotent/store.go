package idempotent

import (
	"context"
	"encoding/json"
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

func (s *Store) claimKey(roundID string) string {
	return fmt.Sprintf("idempotent:round:%s", roundID)
}

func (s *Store) resultKey(roundID string) string {
	return fmt.Sprintf("idempotent:result:%s", roundID)
}

func (s *Store) tokenKey(token string) string {
	return fmt.Sprintf("idempotent:token:%s", token)
}

// GetResult returns cached result for a processed round.
func (s *Store) GetResult(ctx context.Context, roundID string, dest any) (bool, error) {
	raw, err := s.client.Get(ctx, s.resultKey(roundID)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), dest)
}

// SaveResult stores the settlement result for idempotent replay.
func (s *Store) SaveResult(ctx context.Context, roundID string, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.resultKey(roundID), raw, s.ttl).Err()
}

// Claim returns false if the round is already being processed or done.
func (s *Store) Claim(ctx context.Context, roundID string) (bool, error) {
	return s.client.SetNX(ctx, s.claimKey(roundID), "1", s.ttl).Result()
}

// ClaimWithToken binds idempotency token to round and rejects token reuse across rounds.
func (s *Store) ClaimWithToken(ctx context.Context, roundID, token string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("missing idempotency token")
	}

	tokenKey := s.tokenKey(token)
	existing, err := s.client.Get(ctx, tokenKey).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	if err == nil && existing != roundID {
		return false, fmt.Errorf("idempotency token reused")
	}

	claimed, err := s.client.SetNX(ctx, s.claimKey(roundID), "1", s.ttl).Result()
	if err != nil {
		return false, err
	}
	if claimed {
		if err := s.client.Set(ctx, tokenKey, roundID, s.ttl).Err(); err != nil {
			return false, err
		}
	}
	return claimed, nil
}
