package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	applog "fastgame/pkg/log"
	"fastgame/pkg/money"
	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/logx"
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
	UserID     string `json:"userId"`
	Currency   string `json:"currency,omitempty"`
}

type balanceResp struct {
	Balance int64 `json:"balance"`
}

type txReq struct {
	MerchantID string `json:"merchantId"`
	UserID     string `json:"userId"`
	RoundID    string `json:"roundId"`
	Amount     int64  `json:"amount"`
	Reason     string `json:"reason,omitempty"`
}

type txResp struct {
	Balance int64 `json:"balance"`
}

func (c *HTTPClient) GetBalance(ctx context.Context, merchantID, userID, currency string) (money.Amount, error) {
	var resp balanceResp
	err := c.post(ctx, "/api/v1/wallet/balance", balanceReq{
		MerchantID: merchantID,
		UserID:     userID,
		Currency:   currency,
	}, &resp)
	return money.AmountFromMinor(resp.Balance), err
}

type settleReqPayload struct {
	MerchantCode  string `json:"merchantCode"`
	UserID        string `json:"userId"`
	GameCode      string `json:"gameCode"`
	RoundID       string `json:"roundId"`
	TransactionID string `json:"transactionId"`
	Currency      string `json:"currency"`
	BetAmount     int64  `json:"betAmount"`
	WinAmount     int64  `json:"winAmount"`
	Timestamp     int64  `json:"timestamp"`
}

type settleRespPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Balance       int64  `json:"balance"`
		TransactionID string `json:"transactionId"`
	} `json:"data"`
}

func (c *HTTPClient) Settle(ctx context.Context, req SettleReq) (*SettleResult, error) {
	var resp settleRespPayload
	err := c.post(ctx, "/api/v1/spi/wallet/settle", settleReqPayload{
		MerchantCode:  req.MerchantID,
		UserID:        req.UserID,
		GameCode:      req.GameCode,
		RoundID:       req.RoundID,
		TransactionID: req.TransactionID,
		Currency:      req.Currency,
		BetAmount:     req.BetAmount.Minor(),
		WinAmount:     req.WinAmount.Minor(),
		Timestamp:     time.Now().Unix(),
	}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("wallet spi error: code=%d msg=%s", resp.Code, resp.Message)
	}
	return &SettleResult{
		Balance:       money.AmountFromMinor(resp.Data.Balance),
		TransactionID: resp.Data.TransactionID,
	}, nil
}

func (c *HTTPClient) Bet(ctx context.Context, req BetReq) (*Result, error) {
	var resp txResp
	err := c.post(ctx, "/api/v1/wallet/bet", txReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     req.Amount.Minor(),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &Result{Balance: money.AmountFromMinor(resp.Balance)}, nil
}

func (c *HTTPClient) Win(ctx context.Context, req WinReq) (*Result, error) {
	var resp txResp
	err := c.post(ctx, "/api/v1/wallet/win", txReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     req.Amount.Minor(),
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &Result{Balance: money.AmountFromMinor(resp.Balance)}, nil
}

func (c *HTTPClient) Rollback(ctx context.Context, req RollbackReq) error {
	return c.post(ctx, "/api/v1/wallet/rollback", txReq{
		MerchantID: req.MerchantID,
		UserID:     req.UserID,
		RoundID:    req.RoundID,
		Amount:     req.Amount.Minor(),
		Reason:     req.Reason,
	}, &txResp{})
}

type checkTxReq struct {
	MerchantID    string `json:"merchantId"`
	UserID        string `json:"userId"`
	TransactionID string `json:"transactionId"`
}

type checkTxResp struct {
	RoundID   string `json:"roundId"`
	Status    string `json:"status"`
	BetAmount int64  `json:"betAmount"`
	WinAmount int64  `json:"winAmount"`
}

func (c *HTTPClient) CheckTransaction(ctx context.Context, merchantID, userID, roundID string) (*TxCheckResult, error) {
	var resp checkTxResp
	err := c.post(ctx, "/api/v1/wallet/check-transaction", checkTxReq{
		MerchantID:    merchantID,
		UserID:        userID,
		TransactionID: roundID,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &TxCheckResult{
		RoundID:   resp.RoundID,
		Status:    resp.Status,
		BetAmount: money.AmountFromMinor(resp.BetAmount),
		WinAmount: money.AmountFromMinor(resp.WinAmount),
	}, nil
}

func (c *HTTPClient) TransferIn(ctx context.Context, req TransferReq) (*TransferResult, error) {
	return nil, ErrMerchantDisabled
}

func (c *HTTPClient) TransferOut(ctx context.Context, req TransferReq) (*TransferResult, error) {
	return nil, ErrMerchantDisabled
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
	if tid := trace.ID(ctx); tid != "" {
		req.Header.Set(trace.HeaderTraceID, tid)
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	if c.cfg.SignEnabled {
		if err := attachSignHeaders(req, c.cfg.Secret, bodyStr); err != nil {
			return err
		}
	}

	start := time.Now()
	res, err := c.client.Do(req)
	if err != nil {
		if IsTimeoutErr(err) {
			applog.C(ctx).Errorw("wallet_request_timeout",
				logx.Field(applog.KeyPath, path),
				logx.Field(applog.KeyDurationMs, time.Since(start).Milliseconds()),
				logx.Field(applog.KeyErr, err),
			)
			return fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		applog.C(ctx).Errorw("wallet_request_failed",
			logx.Field(applog.KeyPath, path),
			logx.Field(applog.KeyErr, err),
		)
		return err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		applog.C(ctx).Errorw("wallet_response_error",
			logx.Field(applog.KeyPath, path),
			logx.Field(applog.KeyStatus, res.StatusCode),
			logx.Field(applog.KeyDurationMs, time.Since(start).Milliseconds()),
		)
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
