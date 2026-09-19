package wallet

import (
	"context"
	"fmt"

	"fastgame/pkg/money"

	"github.com/redis/go-redis/v9"
)

type TransferWalletClient struct {
	rdb *redis.Client
}

func NewTransferWalletClient(rdb *redis.Client) *TransferWalletClient {
	return &TransferWalletClient{rdb: rdb}
}

func (t *TransferWalletClient) key(merchantID, userID string) string {
	return fmt.Sprintf("wallet:transfer:bal:%s:%s", merchantID, userID)
}

// 基于 Lua 脚本实现原子 Bet + Win 结算
var settleLua = redis.NewScript(`
local key = KEYS[1]
local bet = tonumber(ARGV[1])
local win = tonumber(ARGV[2])

local bal = tonumber(redis.call('GET', key) or "0")
if bal < bet then
    return -1
end

bal = bal - bet + win
redis.call('SET', key, bal)
return bal
`)

// 基于 Lua 脚本实现单边原子扣款 (Bet)
var betLua = redis.NewScript(`
local key = KEYS[1]
local bet = tonumber(ARGV[1])

local bal = tonumber(redis.call('GET', key) or "0")
if bal < bet then
    return -1
end

bal = bal - bet
redis.call('SET', key, bal)
return bal
`)

func (t *TransferWalletClient) GetBalance(ctx context.Context, merchantID, userID, currency string) (money.Amount, error) {
	val, err := t.rdb.Get(ctx, t.key(merchantID, userID)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return money.AmountFromMinor(val), err
}

func (t *TransferWalletClient) Settle(ctx context.Context, req SettleReq) (*SettleResult, error) {
	k := t.key(req.MerchantID, req.UserID)
	res, err := settleLua.Run(ctx, t.rdb, []string{k}, req.BetAmount.Minor(), req.WinAmount.Minor()).Int64()
	if err != nil {
		return nil, err
	}
	if res == -1 {
		return nil, ErrInsufficientFund
	}
	return &SettleResult{
		Balance:       money.AmountFromMinor(res),
		TransactionID: req.TransactionID,
	}, nil
}

func (t *TransferWalletClient) Bet(ctx context.Context, req BetReq) (*Result, error) {
	k := t.key(req.MerchantID, req.UserID)
	res, err := betLua.Run(ctx, t.rdb, []string{k}, req.Amount.Minor()).Int64()
	if err != nil {
		return nil, err
	}
	if res == -1 {
		return nil, ErrInsufficientFund
	}
	return &Result{Balance: money.AmountFromMinor(res)}, nil
}

func (t *TransferWalletClient) Win(ctx context.Context, req WinReq) (*Result, error) {
	k := t.key(req.MerchantID, req.UserID)
	newBal, err := t.rdb.IncrBy(ctx, k, req.Amount.Minor()).Result()
	if err != nil {
		return nil, err
	}
	return &Result{Balance: money.AmountFromMinor(newBal)}, nil
}

func (t *TransferWalletClient) Rollback(ctx context.Context, req RollbackReq) error {
	k := t.key(req.MerchantID, req.UserID)
	return t.rdb.IncrBy(ctx, k, req.Amount.Minor()).Err()
}

func (t *TransferWalletClient) CheckTransaction(ctx context.Context, merchantID, userID, roundID string) (*TxCheckResult, error) {
	return &TxCheckResult{
		RoundID: roundID,
		Status:  TxStatusSettled,
	}, nil
}

// TransferIn 充值上分
func (t *TransferWalletClient) TransferIn(ctx context.Context, req TransferReq) (*TransferResult, error) {
	k := t.key(req.MerchantID, req.UserID)
	newBal, err := t.rdb.IncrBy(ctx, k, req.Amount.Minor()).Result()
	if err != nil {
		return nil, err
	}
	return &TransferResult{
		GameBalance: money.AmountFromMinor(newBal),
		TransferID:  req.TransferID,
	}, nil
}

// TransferOut 提现下分 (原子检查余额并扣减)
func (t *TransferWalletClient) TransferOut(ctx context.Context, req TransferReq) (*TransferResult, error) {
	k := t.key(req.MerchantID, req.UserID)
	res, err := betLua.Run(ctx, t.rdb, []string{k}, req.Amount.Minor()).Int64()
	if err != nil {
		return nil, err
	}
	if res == -1 {
		return nil, ErrInsufficientFund
	}
	return &TransferResult{
		GameBalance: money.AmountFromMinor(res),
		TransferID:  req.TransferID,
	}, nil
}
