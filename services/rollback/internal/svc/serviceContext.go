package svc

import (
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/kafka"
	"fastgame/pkg/ledger"
	"fastgame/pkg/money"
	"fastgame/pkg/wallet"
	"fastgame/services/rollback/internal/config"

	goredis "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config     config.Config
	Writer     *clickhouse.Writer
	Kafka      *kafka.Producer
	Merchants  model.MerchantsModel
	PendingOps model.WalletPendingOpsModel
	PendingTx  model.PendingTransactionsModel
	Wallet     wallet.Client
	DB         sqlx.SqlConn
	// Ledger 账变收口。补偿只要真的动了钱（退款/回滚/补派彩）就必须记账，
	// 否则账本上会缺掉"钱回来了"这一半——对账时只看得见扣款、看不见退款。
	//
	// sink 传 nil（见 NewServiceContext 注释）：rollback 服务没有跑 outbox
	// dispatcher，账变事件的投递留到接入结算/对账时再补；
	// 账变流水本身照常落 MySQL，审计能力不受影响。
	Ledger *ledger.Poster
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
		Kafka:      kafka.NewProducer(c.Kafka.Brokers),
		Merchants:  model.NewMerchantsModel(conn),
		PendingOps: model.NewWalletPendingOpsModel(conn),
		PendingTx:  model.NewPendingTransactionsModel(conn),
		DB:         conn,
		// sink = nil：本服务没有 outbox dispatcher，先把流水落库；
		// 账变事件等结算/对账接入时再一并打开。
		Ledger: ledger.NewPoster(conn, nil),
		Wallet: newWalletClient(c.Wallet, nil),
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
