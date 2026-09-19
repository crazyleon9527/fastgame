package rtpwatchdog

import (
	"context"
	"sync"
	"time"

	"fastgame/pkg/money"
)

// Alert RTP 异常告警（实际返还率以 ppm 表示，180% = 1_800_000）
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

type sample struct {
	merchantCode string
	gameCode     string
	userID       string
	bet          int64
	win          int64
}

// Watchdog 内存滑动窗口 — 全局 10,000 局 + 单玩家若干局
type Watchdog struct {
	mu            sync.Mutex
	global        []sample
	globalMax     int
	playerMax     int
	thresholdPPM  int64
	minSampleBet  int64
	minSamples    int
	playerWindows map[string][]sample
	gameWindows   map[string][]sample
}

type Config struct {
	GlobalMax    int
	PlayerMax    int
	ThresholdPPM int64
	MinSampleBet int64
	// MinSamples 是触发告警所需的最少样本数（局数）。
	//
	// 为什么必须有它：原先硬编码 10 局 + 总投注额 $100 就参与判定，
	// 而本游戏单局派彩的标准差极大（σ≈6 倍投注额，最高 500x）。
	// 10 局窗口里只要出现一次 20x，窗口 RTP 就超过 180% 阈值 —— 正常波动
	// 就会被判成"RTP 漂移"，进而 flagged 游戏、封禁玩家、拒绝全部下注。
	//
	// 样本量的下限由统计决定，不是拍脑袋：
	//
	//	σ_round ≈ 6.04（以投注额为单位）
	//	RTP 的标准误 SE = σ_round / √n
	//	要让 6σ 上界仍低于阈值（180% = 比理论 96% 高 0.84），需
	//	  SE ≤ 0.84/6 = 0.14  →  √n ≥ 6.04/0.14 ≈ 43  →  n ≥ 1858
	//	取 2000：误报率约 P(Z>6.2) ≈ 3e-10，而真故障（例如 864% RTP 的
	//	旧赔付表）会远远越过该阈值，仍然拦得住。
	//
	// 代价是检出延迟从 10 局变成 2000 局 —— 这是统计规律的必然，不是取舍失误。
	MinSamples int
	// AlertCooldown 是同一 key 两次告警之间的最小间隔。
	// 没有它时，窗口一旦越线，之后每一局都会再报一次警、再写一次
	// risk_alerts、再 upsert 一次黑名单（实测 10 局后连续刷屏）。
	// 目前由 RedisWatchdog 实现（跨副本共享）；内存版仅做参数校验。
	AlertCooldown time.Duration
}

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

func New(cfg Config) *Watchdog {
	if cfg.GlobalMax <= 0 {
		cfg = DefaultConfig()
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = DefaultConfig().MinSamples
	}
	if cfg.AlertCooldown <= 0 {
		cfg.AlertCooldown = DefaultConfig().AlertCooldown
	}
	// 窗口装不下判定所需样本时，该维度永远不会告警——静默失效比误报更糟
	if cfg.PlayerMax < cfg.MinSamples {
		cfg.PlayerMax = cfg.MinSamples
	}
	if cfg.GlobalMax < cfg.MinSamples {
		cfg.GlobalMax = cfg.MinSamples
	}
	return &Watchdog{
		globalMax:     cfg.GlobalMax,
		playerMax:     cfg.PlayerMax,
		thresholdPPM:  cfg.ThresholdPPM,
		minSampleBet:  cfg.MinSampleBet,
		minSamples:    cfg.MinSamples,
		playerWindows: make(map[string][]sample),
		gameWindows:   make(map[string][]sample),
	}
}

type RecordInput struct {
	MerchantCode string
	GameCode     string
	UserID       string
	BetMinor     int64
	WinMinor     int64
}

func (w *Watchdog) RecordCtx(_ context.Context, in RecordInput) ([]Alert, error) {
	return w.Record(in), nil
}

func (w *Watchdog) Record(in RecordInput) []Alert {
	if in.BetMinor <= 0 || in.UserID == "" {
		return nil
	}

	s := sample{
		merchantCode: in.MerchantCode,
		gameCode:     in.GameCode,
		userID:       in.UserID,
		bet:          in.BetMinor,
		win:          in.WinMinor,
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.global = appendSample(w.global, s, w.globalMax)
	w.playerWindows[in.UserID] = appendSample(w.playerWindows[in.UserID], s, w.playerMax)
	gameKey := in.MerchantCode + ":" + in.GameCode
	w.gameWindows[gameKey] = appendSample(w.gameWindows[gameKey], s, w.globalMax)

	var alerts []Alert
	if alert, ok := w.evalSamples("user", in.UserID, in.MerchantCode, in.GameCode, in.UserID, w.playerWindows[in.UserID]); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := w.evalSamples("game", gameKey, in.MerchantCode, in.GameCode, "", w.gameWindows[gameKey]); ok {
		alerts = append(alerts, alert)
	}
	return alerts
}

func appendSample(buf []sample, s sample, max int) []sample {
	buf = append(buf, s)
	if len(buf) > max {
		buf = buf[len(buf)-max:]
	}
	return buf
}

func (w *Watchdog) evalSamples(scopeType, scopeValue, merchantCode, gameCode, userID string, samples []sample) (Alert, bool) {
	var totalBet, totalWin int64
	for _, s := range samples {
		totalBet += s.bet
		totalWin += s.win
	}
	if totalBet < w.minSampleBet || len(samples) < w.minSamples {
		return Alert{}, false
	}
	rtpPPM := totalWin * 1_000_000 / totalBet
	if rtpPPM <= w.thresholdPPM {
		return Alert{}, false
	}
	return Alert{
		ScopeType:    scopeType,
		ScopeValue:   scopeValue,
		MerchantCode: merchantCode,
		GameCode:     gameCode,
		UserID:       userID,
		RtpPPM:       rtpPPM,
		TotalBet:     totalBet,
		TotalWin:     totalWin,
		SampleSize:   len(samples),
	}, true
}
