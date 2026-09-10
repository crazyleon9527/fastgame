package rtpwatchdog

import (
	"fmt"
	"sync"

	"fastgame/pkg/money"
)

// Alert RTP 异常告警（实际返还率以 ppm 表示，180% = 1_800_000）
type Alert struct {
	ScopeType    string // user | game
	ScopeValue   string
	MerchantCode string
	GameCode     string
	UserID       uint64
	RtpPPM       int64
	TotalBet     int64
	TotalWin     int64
	SampleSize   int
}

type sample struct {
	merchantCode string
	gameCode     string
	userID       uint64
	bet          int64
	win          int64
}

// Watchdog 内存滑动窗口 — 全局 10,000 局 + 单玩家 200 局
type Watchdog struct {
	mu            sync.Mutex
	global        []sample
	globalMax     int
	playerMax     int
	thresholdPPM  int64
	minSampleBet  int64
	playerWindows map[uint64][]sample
	gameWindows   map[string][]sample
}

type Config struct {
	GlobalMax    int
	PlayerMax    int
	ThresholdPPM int64
	MinSampleBet int64
}

func DefaultConfig() Config {
	return Config{
		GlobalMax:    10000,
		PlayerMax:    200,
		ThresholdPPM: 1800000,
		MinSampleBet: 100 * money.Scale,
	}
}

func New(cfg Config) *Watchdog {
	if cfg.GlobalMax <= 0 {
		cfg = DefaultConfig()
	}
	return &Watchdog{
		globalMax:     cfg.GlobalMax,
		playerMax:     cfg.PlayerMax,
		thresholdPPM:  cfg.ThresholdPPM,
		minSampleBet:  cfg.MinSampleBet,
		playerWindows: make(map[uint64][]sample),
		gameWindows:   make(map[string][]sample),
	}
}

type RecordInput struct {
	MerchantCode string
	GameCode     string
	UserID       uint64
	BetMinor     int64
	WinMinor     int64
}

func (w *Watchdog) Record(in RecordInput) []Alert {
	if in.BetMinor <= 0 {
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
	if alert, ok := w.evalSamples("user", formatUser(in.UserID), in.MerchantCode, in.GameCode, in.UserID, w.playerWindows[in.UserID]); ok {
		alerts = append(alerts, alert)
	}
	if alert, ok := w.evalSamples("game", gameKey, in.MerchantCode, in.GameCode, 0, w.gameWindows[gameKey]); ok {
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

func (w *Watchdog) evalSamples(scopeType, scopeValue, merchantCode, gameCode string, userID uint64, samples []sample) (Alert, bool) {
	var totalBet, totalWin int64
	for _, s := range samples {
		totalBet += s.bet
		totalWin += s.win
	}
	if totalBet < w.minSampleBet || len(samples) < 10 {
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

func formatUser(userID uint64) string {
	return fmt.Sprintf("%d", userID)
}
