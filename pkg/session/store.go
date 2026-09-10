package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 24 * time.Hour

type Data struct {
	Token          string `json:"token"`
	UserID         uint64 `json:"userId"`
	MerchantID     string `json:"merchantId"`
	GameCode       string `json:"gameCode"`
	ServerSeed     string `json:"serverSeed"`
	ServerSeedHash string `json:"serverSeedHash"`
	ClientSeed     string `json:"clientSeed"`
	NextSequence   uint64 `json:"nextSequence"`
}

type Store struct {
	client *redis.Client
	ttl    time.Duration
}

func NewStore(client *redis.Client, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Store{client: client, ttl: ttl}
}

func (s *Store) key(token string) string {
	return fmt.Sprintf("session:token:%s", token)
}

func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func GenerateSeed() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func HashSeed(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Create(ctx context.Context, merchantID string, userID uint64, gameCode, clientSeed string) (*Data, error) {
	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}
	serverSeed, err := GenerateSeed()
	if err != nil {
		return nil, err
	}
	if clientSeed == "" {
		clientSeed, err = GenerateSeed()
		if err != nil {
			return nil, err
		}
	}

	data := &Data{
		Token:          token,
		UserID:         userID,
		MerchantID:     merchantID,
		GameCode:       gameCode,
		ServerSeed:     serverSeed,
		ServerSeedHash: HashSeed(serverSeed),
		ClientSeed:     clientSeed,
		NextSequence:   1,
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err := s.client.Set(ctx, s.key(token), raw, s.ttl).Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) Get(ctx context.Context, token string) (*Data, error) {
	raw, err := s.client.Get(ctx, s.key(token)).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session expired or invalid")
	}
	if err != nil {
		return nil, err
	}
	var data Data
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ConsumeSequence validates monotonic sequence and advances counter atomically.
func (s *Store) ConsumeSequence(ctx context.Context, token string, userID uint64, merchantID, gameCode string, sequenceID uint64) (*Data, error) {
	data, err := s.Get(ctx, token)
	if err != nil {
		return nil, err
	}
	if data.UserID != userID || data.MerchantID != merchantID || data.GameCode != gameCode {
		return nil, fmt.Errorf("session mismatch")
	}
	if sequenceID != data.NextSequence {
		return nil, fmt.Errorf("invalid sequence id")
	}

	data.NextSequence++
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err := s.client.Set(ctx, s.key(token), raw, s.ttl).Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) UpdateClientSeed(ctx context.Context, token, clientSeed string) error {
	if clientSeed == "" {
		return nil
	}
	data, err := s.Get(ctx, token)
	if err != nil {
		return err
	}
	data.ClientSeed = clientSeed
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(token), raw, s.ttl).Err()
}
