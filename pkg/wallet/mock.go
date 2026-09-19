package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"fastgame/pkg/money"
)

type roundState struct {
	status    string
	betAmount money.Amount
	winAmount money.Amount
}

type MockClient struct {
	mu       sync.Mutex
	balances map[string]money.Amount
	rounds   map[string]roundState
}

func NewMockClient(initialBalance money.Amount) *MockClient {
	if initialBalance <= 0 {
		initialBalance = money.FromMajor(10000)
	}
	return &MockClient{
		balances: map[string]money.Amount{"default": initialBalance},
		rounds:   map[string]roundState{},
	}
}

func (m *MockClient) accountKey(merchantID string, userID string) string {
	return fmt.Sprintf("%s:%s", merchantID, userID)
}

func (m *MockClient) GetBalance(ctx context.Context, merchantID, userID, currency string) (money.Amount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(merchantID, userID)
	if bal, ok := m.balances[key]; ok {
		return bal, nil
	}
	m.balances[key] = money.FromMajor(10000)
	return m.balances[key], nil
}

func (m *MockClient) Settle(ctx context.Context, req SettleReq) (*SettleResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	bal, ok := m.balances[key]
	if !ok {
		bal = money.FromMajor(10000)
	}
	if bal < req.BetAmount {
		return nil, ErrInsufficientFund
	}
	bal = bal - req.BetAmount + req.WinAmount
	m.balances[key] = bal
	m.rounds[req.RoundID] = roundState{
		status:    TxStatusSettled,
		betAmount: req.BetAmount,
		winAmount: req.WinAmount,
	}
	return &SettleResult{
		Balance:       bal,
		TransactionID: req.TransactionID,
	}, nil
}

func (m *MockClient) Bet(ctx context.Context, req BetReq) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	bal, ok := m.balances[key]
	if !ok {
		bal = money.FromMajor(10000)
	}
	if bal < req.Amount {
		return nil, ErrInsufficientFund
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

func (m *MockClient) CheckTransaction(ctx context.Context, merchantID, userID, roundID string) (*TxCheckResult, error) {
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

func (m *MockClient) TransferIn(ctx context.Context, req TransferReq) (*TransferResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	m.balances[key] += req.Amount
	return &TransferResult{
		GameBalance: m.balances[key],
		TransferID:  req.TransferID,
	}, nil
}

func (m *MockClient) TransferOut(ctx context.Context, req TransferReq) (*TransferResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.accountKey(req.MerchantID, req.UserID)
	if m.balances[key] < req.Amount {
		return nil, ErrInsufficientFund
	}
	m.balances[key] -= req.Amount
	return &TransferResult{
		GameBalance: m.balances[key],
		TransferID:  req.TransferID,
	}, nil
}

// -----------------------------------------------------------------------------
// HTTP 服务端 Handler 实现 (供独立 Mock HTTP 服务直接挂载)
// -----------------------------------------------------------------------------

func (m *MockClient) RegisterHTTPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/wallet/balance", m.handleHTTPBalance)
	mux.HandleFunc("/api/v1/spi/wallet/settle", m.handleHTTPSettle)
	mux.HandleFunc("/api/v1/wallet/bet", m.handleHTTPBet)
	mux.HandleFunc("/api/v1/wallet/win", m.handleHTTPWin)
	mux.HandleFunc("/api/v1/wallet/rollback", m.handleHTTPRollback)
	mux.HandleFunc("/api/v1/wallet/check-transaction", m.handleHTTPCheckTransaction)
}

func (m *MockClient) handleHTTPBalance(w http.ResponseWriter, r *http.Request) {
	var req balanceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bal, err := m.GetBalance(r.Context(), req.MerchantID, req.UserID, req.Currency)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(balanceResp{Balance: bal.Minor()})
}

func (m *MockClient) handleHTTPSettle(w http.ResponseWriter, r *http.Request) {
	var req settleReqPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := m.Settle(r.Context(), SettleReq{
		MerchantID:    req.MerchantCode,
		UserID:        req.UserID,
		GameCode:      req.GameCode,
		RoundID:       req.RoundID,
		TransactionID: req.TransactionID,
		Currency:      req.Currency,
		BetAmount:     money.AmountFromMinor(req.BetAmount),
		WinAmount:     money.AmountFromMinor(req.WinAmount),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settleRespPayload{
		Code:    0,
		Message: "success",
		Data: struct {
			Balance       int64  `json:"balance"`
			TransactionID string `json:"transactionId"`
		}{
			Balance:       res.Balance.Minor(),
			TransactionID: res.TransactionID,
		},
	})
}

func (m *MockClient) handleHTTPBet(w http.ResponseWriter, r *http.Request) {
	var req txReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := m.Bet(r.Context(), BetReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     money.AmountFromMinor(req.Amount),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(txResp{Balance: res.Balance.Minor()})
}

func (m *MockClient) handleHTTPWin(w http.ResponseWriter, r *http.Request) {
	var req txReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := m.Win(r.Context(), WinReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     money.AmountFromMinor(req.Amount),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(txResp{Balance: res.Balance.Minor()})
}

func (m *MockClient) handleHTTPRollback(w http.ResponseWriter, r *http.Request) {
	var req txReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err := m.Rollback(r.Context(), RollbackReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     money.AmountFromMinor(req.Amount),
		Reason:     req.Reason,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(txResp{})
}

func (m *MockClient) handleHTTPCheckTransaction(w http.ResponseWriter, r *http.Request) {
	var req checkTxReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := m.CheckTransaction(r.Context(), req.MerchantID, req.UserID, req.TransactionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(checkTxResp{
		RoundID:   res.RoundID,
		Status:    res.Status,
		BetAmount: res.BetAmount.Minor(),
		WinAmount: res.WinAmount.Minor(),
	})
}
