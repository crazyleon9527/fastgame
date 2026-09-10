package wallet

import (
	"context"
	"errors"
	"testing"
	"time"

	"fastgame/pkg/money"
)

type slowMock struct {
	delay time.Duration
	*MockClient
}

func (s *slowMock) GetBalance(ctx context.Context, merchantID string, userID uint64) (money.Amount, error) {
	time.Sleep(s.delay)
	return s.MockClient.GetBalance(ctx, merchantID, userID)
}

func TestBreakerRejectsSlowBalance(t *testing.T) {
	inner := &slowMock{delay: 600 * time.Millisecond, MockClient: NewMockClient(money.FromMajor(100))}
	client := NewBreakerClient(inner, BreakerConfig{SlowThreshold: 500 * time.Millisecond})

	_, err := client.GetBalance(context.Background(), "m001", 1)
	if !errors.Is(err, ErrSlowResponse) {
		t.Fatalf("expected slow response error, got %v", err)
	}
}
