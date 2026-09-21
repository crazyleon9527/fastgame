package rtpwatchdog

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"fastgame/internal/model"
	"fastgame/pkg/security"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const suspendTTL = 24 * time.Hour

type Enforcer struct {
	redis       *redis.Client
	blacklistDB model.RiskBlacklistModel
	blacklist   *security.Blacklist
	alerts      model.RiskAlertsModel
}

func NewEnforcer(
	rdb *redis.Client,
	blacklistDB model.RiskBlacklistModel,
	blacklist *security.Blacklist,
	alerts model.RiskAlertsModel,
) *Enforcer {
	return &Enforcer{
		redis:       rdb,
		blacklistDB: blacklistDB,
		blacklist:   blacklist,
		alerts:      alerts,
	}
}

func (e *Enforcer) Handle(ctx context.Context, alert Alert) error {
	action := "user_suspended"
	if alert.ScopeType == "game" {
		action = "game_flagged"
	}

	// 1. 记录风控告警审计记录
	if err := e.alerts.Insert(ctx, &model.RiskAlert{
		AlertType:    "rtp_drift",
		ScopeType:    alert.ScopeType,
		ScopeValue:   alert.ScopeValue,
		MerchantCode: alert.MerchantCode,
		GameCode:     alert.GameCode,
		RtpPPM:       alert.RtpPPM,
		TotalBet:     alert.TotalBet,
		TotalWin:     alert.TotalWin,
		SampleSize:   int64(alert.SampleSize),
		ActionTaken:  action,
		Status:       "open",
	}); err != nil {
		logx.WithContext(ctx).Errorf("risk alert insert failed: %v", err)
	}

	// 2. 玩家级处置：加入多租户挂起标记与自动过期黑名单
	if alert.ScopeType == "user" && alert.UserID != "" {
		// Redis 熔断挂起 Key：带上 merchantCode 进行严格隔离
		key := fmt.Sprintf("rtp:suspend:user:%s:%s", alert.MerchantCode, alert.UserID)
		if err := e.redis.Set(ctx, key, "1", suspendTTL).Err(); err != nil {
			return err
		}

		reason := fmt.Sprintf("auto rtp watchdog drift merchant=%s ppm=%d samples=%d", alert.MerchantCode, alert.RtpPPM, alert.SampleSize)
		row := &model.RiskBlacklist{
			ListType:  model.BlacklistTypeUserID,
			ListValue: alert.UserID,
			Reason:    reason,
			Status:    1,
			ExpiresAt: sql.NullTime{Time: time.Now().Add(suspendTTL), Valid: true},
		}

		if _, err := e.blacklistDB.Upsert(ctx, row); err != nil {
			logx.WithContext(ctx).Errorf("auto blacklist user failed: %v", err)
		} else if err := e.blacklist.SyncItem(ctx, row); err != nil {
			logx.WithContext(ctx).Errorf("sync blacklist redis failed: %v", err)
		}

		logx.WithContext(ctx).Errorf("[RTP-WATCHDOG] user suspended: merchant=%s userId=%s rtpPPM=%d totalBet=%d totalWin=%d",
			alert.MerchantCode, alert.UserID, alert.RtpPPM, alert.TotalBet, alert.TotalWin)
	}

	// 3. 游戏级处置：标记异常游戏
	if alert.ScopeType == "game" {
		key := fmt.Sprintf("rtp:flag:game:%s:%s", alert.MerchantCode, alert.GameCode)
		if err := e.redis.Set(ctx, key, "1", suspendTTL).Err(); err != nil {
			return err
		}
		logx.WithContext(ctx).Errorf("[RTP-WATCHDOG] game flagged: merchant=%s game=%s rtpPPM=%d totalBet=%d totalWin=%d",
			alert.MerchantCode, alert.GameCode, alert.RtpPPM, alert.TotalBet, alert.TotalWin)
	}

	return nil
}
