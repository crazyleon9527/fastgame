package svc

import (
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/money"
	"fastgame/pkg/wallet"
	"fastgame/services/rollback/internal/config"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config     config.Config
	Writer     *clickhouse.Writer
	Merchants  model.MerchantsModel
	PendingOps model.WalletPendingOpsModel
	PendingTx  model.PendingTransactionsModel
	Wallet     wallet.Client
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	writer, err := clickhouse.NewWriterWithAuth(
		c.ClickHouse.Addr,
		c.ClickHouse.Database,
		c.ClickHouse.User,
		c.ClickHouse.Password,
	)
	if err != nil {
		return nil, err
	}

	conn := sqlx.NewMysql(c.MySQL.DataSource)
	return &ServiceContext{
		Config:     c,
		Writer:     writer,
		Merchants:  model.NewMerchantsModel(conn),
		PendingOps: model.NewWalletPendingOpsModel(conn),
		PendingTx:  model.NewPendingTransactionsModel(conn),
		Wallet:     newWalletClient(c.Wallet, nil),
	}, nil
}

func newWalletClient(cfg config.WalletConf, _ *goredis.Client) wallet.Client {
	var inner wallet.Client
	if cfg.Mock || cfg.BaseURL == "" {
		inner = wallet.NewMockClient(money.FromMajor(10000))
	} else {
		timeout, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			timeout = 5 * time.Second
		}
		inner = wallet.NewHTTPClient(wallet.HTTPConfig{
			BaseURL:              cfg.BaseURL,
			APIKey:               cfg.APIKey,
			Secret:               cfg.SignSecret,
			SignEnabled:          cfg.SignEnabled,
			VerifyResponse:       cfg.VerifyResponse,
			ResponseTimestampWin: cfg.ResponseTimestampWindow(),
			Timeout:              timeout,
		})
	}
	return wallet.NewBreakerClient(inner, wallet.BreakerConfig{
		SlowThreshold: cfg.SlowThreshold(),
	})
}
