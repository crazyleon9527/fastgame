package svc

import (
	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/services/consumer/internal/config"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config    config.Config
	Writer    *clickhouse.Writer
	Merchants model.MerchantsModel
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
		Config:    c,
		Writer:    writer,
		Merchants: model.NewMerchantsModel(conn),
	}, nil
}
