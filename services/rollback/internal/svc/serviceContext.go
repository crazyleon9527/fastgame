package svc

import (
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/wallet"
	"fastgame/services/rollback/internal/config"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config    config.Config
	Writer    *clickhouse.Writer
	Merchants model.MerchantsModel
	Wallet    wallet.Client
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

	return &ServiceContext{
		Config:    c,
		Writer:    writer,
		Merchants: model.NewMerchantsModel(sqlx.NewMysql(c.MySQL.DataSource)),
		Wallet:    newWalletClient(c.Wallet),
	}, nil
}

func newWalletClient(cfg config.WalletConf) wallet.Client {
	if cfg.Mock || cfg.BaseURL == "" {
		return wallet.NewMockClient(10000)
	}
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		timeout = 5 * time.Second
	}
	return wallet.NewHTTPClient(wallet.HTTPConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Timeout: timeout,
	})
}
