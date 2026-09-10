package svc

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/security"
	"fastgame/services/admin/internal/config"
	"fastgame/services/admin/internal/middleware"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	Config         config.Config
	AuthMiddleware rest.Middleware
	AdminUsers     model.AdminUsersModel
	AdminAuth      model.AdminAuthModel
	Merchants      model.MerchantsModel
	RiskAlerts     model.RiskAlertsModel
	GameConfigs    model.GameConfigsModel
	RiskBlacklist  model.RiskBlacklistModel
	Blacklist      *security.Blacklist
	IPWhitelist    *security.IPWhitelist
	Reporter       *clickhouse.Writer
	PendingTx      model.PendingTransactionsModel
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	reporter, err := clickhouse.NewWriterWithAuth(c.CH.Addr, c.CH.Database, c.CH.User, c.CH.Password)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
	blacklistModel := model.NewRiskBlacklistModel(conn)
	merchantsModel := model.NewMerchantsModel(conn)
	blacklist := security.NewBlacklist(rdb)

	svcCtx := &ServiceContext{
		Config:         c,
		AuthMiddleware: middleware.NewAuthMiddleware().Handle,
		AdminUsers:     model.NewAdminUsersModel(conn),
		AdminAuth:      model.NewAdminAuthModel(conn),
		Merchants:      merchantsModel,
		RiskAlerts:     model.NewRiskAlertsModel(conn),
		GameConfigs:    model.NewGameConfigsModel(conn),
		RiskBlacklist:  blacklistModel,
		Blacklist:      blacklist,
		IPWhitelist:    security.NewIPWhitelist(merchantsModel, rdb),
		Reporter:  reporter,
		PendingTx: model.NewPendingTransactionsModel(conn),
	}

	bootstrapBlacklist(rdb, blacklistModel)
	return svcCtx, nil
}

func bootstrapBlacklist(rdb *redis.Client, blacklistModel model.RiskBlacklistModel) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items, err := blacklistModel.ListAllActive(ctx)
	if err != nil {
		logx.Errorf("admin bootstrap blacklist failed: %v", err)
		return
	}
	if err := security.NewBlacklist(rdb).SyncAll(ctx, items); err != nil {
		logx.Errorf("admin sync blacklist failed: %v", err)
	}
}
