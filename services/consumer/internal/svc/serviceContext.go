package svc

import (
	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/rtpwatchdog"
	"fastgame/pkg/security"
	"fastgame/services/consumer/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config    config.Config
	Writer    *clickhouse.Writer
	Merchants model.MerchantsModel
	Watchdog  *rtpwatchdog.Watchdog
	Enforcer  *rtpwatchdog.Enforcer
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
	rdb := redis.NewClient(&redis.Options{Addr: c.Redis.Addr})
	blacklistModel := model.NewRiskBlacklistModel(conn)
	blacklist := security.NewBlacklist(rdb)

	watchCfg := rtpwatchdog.DefaultConfig()
	if c.RtpWatch.GlobalMax > 0 {
		watchCfg.GlobalMax = c.RtpWatch.GlobalMax
	}
	if c.RtpWatch.PlayerMax > 0 {
		watchCfg.PlayerMax = c.RtpWatch.PlayerMax
	}
	if c.RtpWatch.ThresholdPPM > 0 {
		watchCfg.ThresholdPPM = c.RtpWatch.ThresholdPPM
	}

	return &ServiceContext{
		Config:    c,
		Writer:    writer,
		Merchants: model.NewMerchantsModel(conn),
		Watchdog:  rtpwatchdog.New(watchCfg),
		Enforcer:  rtpwatchdog.NewEnforcer(rdb, blacklistModel, blacklist, model.NewRiskAlertsModel(conn)),
	}, nil
}
