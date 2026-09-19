package svc

import (
	"context"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/clickhouse"
	"fastgame/pkg/outbox"
	"fastgame/pkg/rtpwatchdog"
	"fastgame/pkg/security"
	"fastgame/services/consumer/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type RtpRecorder interface {
	Record(ctx context.Context, in rtpwatchdog.RecordInput) ([]rtpwatchdog.Alert, error)
}

type ServiceContext struct {
	Config    config.Config
	Writer    *clickhouse.Writer
	Merchants model.MerchantsModel
	Recorder  RtpRecorder
	Enforcer  *rtpwatchdog.Enforcer

	// DB 与 Idem 用于消费端 DB 兜底幂等：
	// outbox 的投递保证是 at-least-once，重复事件必须在这里被拦住，
	// 否则重复写 ClickHouse 会让 RTP 统计与账单翻倍。
	DB   sqlx.SqlConn
	Idem *outbox.Idempotency
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
	if c.RtpWatch.MinSamples > 0 {
		watchCfg.MinSamples = c.RtpWatch.MinSamples
	}
	if c.RtpWatch.AlertCooldown != "" {
		if d, err := time.ParseDuration(c.RtpWatch.AlertCooldown); err == nil && d > 0 {
			watchCfg.AlertCooldown = d
		} else {
			logx.Errorf("RtpWatch.AlertCooldown=%q 解析失败或非正数，用默认值 %s",
				c.RtpWatch.AlertCooldown, watchCfg.AlertCooldown)
		}
	}
	// 窗口必须能装下判定所需的最少样本，否则该维度永远不会告警
	if watchCfg.PlayerMax < watchCfg.MinSamples {
		watchCfg.PlayerMax = watchCfg.MinSamples
	}
	if watchCfg.GlobalMax < watchCfg.MinSamples {
		watchCfg.GlobalMax = watchCfg.MinSamples
	}
	logx.Infof("[RTP-WATCHDOG] 生效配置: threshold=%dppm minSamples=%d minSampleBet=%d playerMax=%d globalMax=%d cooldown=%s",
		watchCfg.ThresholdPPM, watchCfg.MinSamples, watchCfg.MinSampleBet,
		watchCfg.PlayerMax, watchCfg.GlobalMax, watchCfg.AlertCooldown)

	var recorder RtpRecorder = rtpwatchdog.NewRedis(rdb, watchCfg)
	if !c.RtpWatch.UseRedis {
		recorder = &memoryRtpAdapter{wd: rtpwatchdog.New(watchCfg)}
	}

	return &ServiceContext{
		Config:    c,
		Writer:    writer,
		Merchants: model.NewMerchantsModel(conn),
		Recorder:  recorder,
		Enforcer:  rtpwatchdog.NewEnforcer(rdb, blacklistModel, blacklist, model.NewRiskAlertsModel(conn)),
		DB:        conn,
		Idem:      outbox.NewIdempotency(conn),
	}, nil
}

type memoryRtpAdapter struct {
	wd *rtpwatchdog.Watchdog
}

func (m *memoryRtpAdapter) Record(ctx context.Context, in rtpwatchdog.RecordInput) ([]rtpwatchdog.Alert, error) {
	return m.wd.RecordCtx(ctx, in)
}
