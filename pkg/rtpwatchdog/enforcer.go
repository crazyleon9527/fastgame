package rtpwatchdog

import (
	"context"
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
		logx.Errorf("risk alert insert failed: %v", err)
	}

	if alert.ScopeType == "user" && alert.UserID > 0 {
		key := fmt.Sprintf("rtp:suspend:user:%d", alert.UserID)
		if err := e.redis.Set(ctx, key, "1", suspendTTL).Err(); err != nil {
			return err
		}
		reason := fmt.Sprintf("auto rtp watchdog ppm=%d samples=%d", alert.RtpPPM, alert.SampleSize)
		row := &model.RiskBlacklist{
			ListType:  model.BlacklistTypeUserID,
			ListValue: alert.ScopeValue,
			Reason:    reason,
			Status:    1,
		}
		if _, err := e.blacklistDB.Upsert(ctx, row); err != nil {
			logx.Errorf("auto blacklist user failed: %v", err)
		} else if err := e.blacklist.SyncItem(ctx, row); err != nil {
			logx.Errorf("sync blacklist redis failed: %v", err)
		}
		logx.Errorf("[RTP-WATCHDOG] user suspended: userId=%s rtpPPM=%d", alert.ScopeValue, alert.RtpPPM)
	}

	if alert.ScopeType == "game" {
		key := fmt.Sprintf("rtp:flag:game:%s:%s", alert.MerchantCode, alert.GameCode)
		if err := e.redis.Set(ctx, key, "1", suspendTTL).Err(); err != nil {
			return err
		}
		logx.Errorf("[RTP-WATCHDOG] game flagged: game=%s rtpPPM=%d", alert.ScopeValue, alert.RtpPPM)
	}
	return nil
}
