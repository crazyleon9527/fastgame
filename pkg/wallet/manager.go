package wallet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/money"

	"github.com/redis/go-redis/v9"
)

type WalletManager struct {
	merchants model.MerchantsModel
	redis     *redis.Client

	mu       sync.RWMutex
	seamless map[string]Client
	transfer Client
}

func NewWalletManager(merchants model.MerchantsModel, rdb *redis.Client, transferClient Client) *WalletManager {
	return &WalletManager{
		merchants: merchants,
		redis:     rdb,
		seamless:  make(map[string]Client),
		transfer:  transferClient,
	}
}

func (m *WalletManager) getClient(ctx context.Context, merchantCode string) (Client, error) {
	m.mu.RLock()
	cli, ok := m.seamless[merchantCode]
	m.mu.RUnlock()
	if ok {
		return cli, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if cli, ok = m.seamless[merchantCode]; ok {
		return cli, nil
	}

	// 1. 查询商户密钥配置
	secrets, err := m.merchants.FindSecretsByMerchantCode(ctx, merchantCode)
	if err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	secretKey := ""
	if secrets.PrivateKey.Valid {
		secretKey = secrets.PrivateKey.String
	}

	// 2. 实例化免转 HTTP 客户端
	rawHTTP := NewHTTPClient(HTTPConfig{
		BaseURL:     "", // 预留或从商户扩展信息/配置中注入
		Secret:      secretKey,
		SignEnabled: secretKey != "",
		Timeout:     2000 * time.Millisecond,
	})

	// 3. 装饰断路器
	decorated := NewBreakerClient(rawHTTP, BreakerConfig{
		SlowThreshold: 1500 * time.Millisecond,
		Redis:         m.redis,
	})

	m.seamless[merchantCode] = decorated
	return decorated, nil
}

func (m *WalletManager) GetBalance(ctx context.Context, merchantID, userID, currency string) (money.Amount, error) {
	cli, err := m.getClient(ctx, merchantID)
	if err != nil {
		return 0, err
	}
	return cli.GetBalance(ctx, merchantID, userID, currency)
}

func (m *WalletManager) Settle(ctx context.Context, req SettleReq) (*SettleResult, error) {
	cli, err := m.getClient(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}
	return cli.Settle(ctx, req)
}

func (m *WalletManager) Bet(ctx context.Context, req BetReq) (*Result, error) {
	cli, err := m.getClient(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}
	return cli.Bet(ctx, req)
}

func (m *WalletManager) Win(ctx context.Context, req WinReq) (*Result, error) {
	cli, err := m.getClient(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}
	return cli.Win(ctx, req)
}

func (m *WalletManager) Rollback(ctx context.Context, req RollbackReq) error {
	cli, err := m.getClient(ctx, req.MerchantID)
	if err != nil {
		return err
	}
	return cli.Rollback(ctx, req)
}

func (m *WalletManager) CheckTransaction(ctx context.Context, merchantID, userID, roundID string) (*TxCheckResult, error) {
	cli, err := m.getClient(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	return cli.CheckTransaction(ctx, merchantID, userID, roundID)
}

func (m *WalletManager) TransferIn(ctx context.Context, req TransferReq) (*TransferResult, error) {
	if m.transfer != nil {
		return m.transfer.TransferIn(ctx, req)
	}
	return nil, ErrMerchantDisabled
}

func (m *WalletManager) TransferOut(ctx context.Context, req TransferReq) (*TransferResult, error) {
	if m.transfer != nil {
		return m.transfer.TransferOut(ctx, req)
	}
	return nil, ErrMerchantDisabled
}
