package rtpwatchdog

import (
	"time"

	"fastgame/pkg/money"
)

// Alert RTP 异常告警结构
type Alert struct {
	ScopeType    string // user | game
	ScopeValue   string
	MerchantCode string
	GameCode     string
	UserID       string
	RtpPPM       int64
	TotalBet     int64
	TotalWin     int64
	SampleSize   int
}

// Config 告警与样本统计配置
type Config struct {
	GlobalMax     int           // 全局/游戏维度滑动窗口最大样本数
	PlayerMax     int           // 玩家维度滑动窗口最大样本数
	ThresholdPPM  int64         // 触发告警的 RTP 阈值 (PPM: 1800000 = 180%)
	MinSampleBet  int64         // 触发告警所需的最小累计投注额 (Minor Units)
	MinSamples    int           // 触发告警所需的最少样本局数 (基于 6σ 统计推导，默认 2000 局)
	AlertCooldown time.Duration // 两次触发告警之间的最小冷却时间，防止日志与告警风暴
}

// DefaultConfig 默认生产配置
func DefaultConfig() Config {
	return Config{
		GlobalMax:     10000,
		PlayerMax:     2000,
		ThresholdPPM:  1800000, // 180%
		MinSampleBet:  100 * money.Scale,
		MinSamples:    2000,
		AlertCooldown: 10 * time.Minute,
	}
}

// RecordInput 每一注的结算输入
type RecordInput struct {
	MerchantCode string
	GameCode     string
	UserID       string
	BetMinor     int64
	WinMinor     int64
}
