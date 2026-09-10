package wallet

import (
	"context"
	"fmt"
	"sync"
)

type roundState struct {
	status    string
	betAmount float64
	winAmount float64
}

// MockClient is a dev stub for Seamless Wallet integration.
type MockClient struct {
	mu       sync.Mutex
	balances map[string]float64
	rounds   map[string]roundState
}

func NewMockClient(initialBalance float64) *MockClient {
	return &MockClient{
		balances: map[string]float64{"default": initialBalance},
		rounds:   map[string]roundState{},
	}
}

func (m *MockClient) accountKey(merchantID string, userID uint64) string {
	return fmt.Sprintf("%s:%d", merchantID, userID)
}

func (m *MockClient) GetBalance(ctx context.Context, merchantID string, userID uint64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(merchantID, userID)
	if bal, ok := m.balances[key]; ok {
		return bal, nil
	}
	m.balances[key] = 10000
	return m.balances[key], nil
}

func (m *MockClient) Bet(ctx context.Context, req BetReq) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	bal := m.balances[key]
	if bal == 0 {
		bal = 10000
	}
	if bal < req.Amount {
		return nil, fmt.Errorf("insufficient balance")
	}
	bal -= req.Amount
	m.balances[key] = bal
	m.rounds[req.RoundID] = roundState{status: TxStatusBetOnly, betAmount: req.Amount}
	return &Result{Balance: bal}, nil
}

func (m *MockClient) Win(ctx context.Context, req WinReq) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	bal := m.balances[key]
	bal += req.Amount
	m.balances[key] = bal
	if rs, ok := m.rounds[req.RoundID]; ok {
		rs.status = TxStatusSettled
		rs.winAmount = req.Amount
		m.rounds[req.RoundID] = rs
	} else {
		m.rounds[req.RoundID] = roundState{status: TxStatusSettled, winAmount: req.Amount}
	}
	return &Result{Balance: bal}, nil
}

func (m *MockClient) Rollback(ctx context.Context, req RollbackReq) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	m.balances[key] += req.Amount
	if rs, ok := m.rounds[req.RoundID]; ok {
		rs.status = TxStatusRolledBack
		m.rounds[req.RoundID] = rs
	} else {
		m.rounds[req.RoundID] = roundState{status: TxStatusRolledBack, betAmount: req.Amount}
	}
	return nil
}

func (m *MockClient) CheckTransaction(ctx context.Context, merchantID string, userID uint64, roundID string) (*TxCheckResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rs, ok := m.rounds[roundID]
	if !ok {
		return &TxCheckResult{RoundID: roundID, Status: TxStatusNotFound}, nil
	}
	return &TxCheckResult{
		RoundID:   roundID,
		Status:    rs.status,
		BetAmount: rs.betAmount,
		WinAmount: rs.winAmount,
	}, nil
}
