package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"strconv"
)

type HTTPConfig struct {
	BaseURL              string
	APIKey               string
	Secret               string
	SignEnabled          bool
	VerifyResponse       bool
	ResponseTimestampWin time.Duration
	Timeout              time.Duration
}

type HTTPClient struct {
	cfg    HTTPConfig
	client *http.Client
}

func NewHTTPClient(cfg HTTPConfig) *HTTPClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.ResponseTimestampWin <= 0 {
		cfg.ResponseTimestampWin = 60 * time.Second
	}
	return &HTTPClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

type balanceReq struct {
	MerchantID string `json:"merchantId"`
	UserID     uint64 `json:"userId"`
}

type balanceResp struct {
	Balance float64 `json:"balance"`
}

type txReq struct {
	MerchantID string  `json:"merchantId"`
	UserID     uint64  `json:"userId"`
	RoundID    string  `json:"roundId"`
	Amount     float64 `json:"amount"`
	Reason     string  `json:"reason,omitempty"`
}

type txResp struct {
	Balance float64 `json:"balance"`
}

func (c *HTTPClient) GetBalance(ctx context.Context, merchantID string, userID uint64) (float64, error) {
	var resp balanceResp
	err := c.post(ctx, "/api/v1/wallet/balance", balanceReq{
		MerchantID: merchantID,
		UserID:     userID,
	}, &resp)
	return resp.Balance, err
}

func (c *HTTPClient) Bet(ctx context.Context, req BetReq) (*Result, error) {
	var resp txResp
	err := c.post(ctx, "/api/v1/wallet/bet", txReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     req.Amount,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &Result{Balance: resp.Balance}, nil
}

func (c *HTTPClient) Win(ctx context.Context, req WinReq) (*Result, error) {
	var resp txResp
	err := c.post(ctx, "/api/v1/wallet/win", txReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     req.Amount,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &Result{Balance: resp.Balance}, nil
}

func (c *HTTPClient) Rollback(ctx context.Context, req RollbackReq) error {
	return c.post(ctx, "/api/v1/wallet/rollback", txReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     req.Amount,
		Reason:     req.Reason,
	}, &txResp{})
}

type checkTxReq struct {
	MerchantID string `json:"merchantId"`
	UserID     uint64 `json:"userId"`
	RoundID    string `json:"roundId"`
}

type checkTxResp struct {
	RoundID   string  `json:"roundId"`
	Status    string  `json:"status"`
	BetAmount float64 `json:"betAmount"`
	WinAmount float64 `json:"winAmount"`
}

func (c *HTTPClient) CheckTransaction(ctx context.Context, merchantID string, userID uint64, roundID string) (*TxCheckResult, error) {
	var resp checkTxResp
	err := c.post(ctx, "/api/v1/wallet/check-transaction", checkTxReq{
		MerchantID: merchantID,
		UserID:     userID,
		RoundID:    roundID,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &TxCheckResult{
		RoundID:   resp.RoundID,
		Status:    resp.Status,
		BetAmount: resp.BetAmount,
		WinAmount: resp.WinAmount,
	}, nil
}

func (c *HTTPClient) post(ctx context.Context, path string, body any, dest any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	bodyStr := string(raw)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	if c.cfg.SignEnabled {
		if err := attachSignHeaders(req, c.cfg.Secret, bodyStr); err != nil {
			return err
		}
	}

	res, err := c.client.Do(req)
	if err != nil {
		if IsTimeoutErr(err) {
			return fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		return err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("wallet api %s: status=%d body=%s", path, res.StatusCode, string(respBody))
	}

	if c.cfg.SignEnabled && c.cfg.VerifyResponse {
		if err := verifyResponseSign(c.cfg.Secret, res, respBody); err != nil {
			return fmt.Errorf("wallet response signature invalid: %w", err)
		}
		if err := validateResponseTimestamp(res.Header.Get(HeaderTimestamp), c.cfg.ResponseTimestampWin); err != nil {
			return err
		}
	}

	if dest == nil {
		return nil
	}
	return json.Unmarshal(respBody, dest)
}

func validateResponseTimestamp(tsHeader string, window time.Duration) error {
	if tsHeader == "" {
		return fmt.Errorf("missing response timestamp")
	}
	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid response timestamp")
	}
	now := time.Now().Unix()
	if ts < now-int64(window.Seconds()) || ts > now+30 {
		return fmt.Errorf("response timestamp expired")
	}
	return nil
}
