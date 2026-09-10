package svc

import (
	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/services/admin/internal/config"
	"fastgame/services/admin/internal/middleware"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	Config         config.Config
	AuthMiddleware rest.Middleware
	AdminUsers     model.AdminUsersModel
	Merchants      model.MerchantsModel
	GameConfigs    model.GameConfigsModel
	Reporter       *clickhouse.Writer
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	reporter, err := clickhouse.NewWriterWithAuth(c.CH.Addr, c.CH.Database, c.CH.User, c.CH.Password)
	if err != nil {
		return nil, err
	}

	return &ServiceContext{
		Config:         c,
		AuthMiddleware: middleware.NewAuthMiddleware().Handle,
		AdminUsers:     model.NewAdminUsersModel(conn),
		Merchants:      model.NewMerchantsModel(conn),
		GameConfigs:    model.NewGameConfigsModel(conn),
		Reporter:       reporter,
	}, nil
}
