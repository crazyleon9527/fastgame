package rtpwatchdog

import (
	"context"
	"testing"
	"time"

	"fastgame/pkg/money"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestWatchdogTriggersHighRtp —— 真正异常时仍然要报出来。
func TestWatchdogTriggersHighRtp(t *testing.T) {
	w := New(Config{GlobalMax: 100, PlayerMax: 100, MinSamples: 20,
		ThresholdPPM: 1800000, MinSampleBet: money.Scale})

	var alerts []Alert
	for i := 0; i < 20; i++ {
		alerts = append(alerts, w.Record(RecordInput{
			MerchantCode: "m001",
			GameCode:     "fishing",
			UserID:       "10001",
			BetMinor:     10 * money.Scale,
			WinMinor:     20 * money.Scale, // 200% RTP
		})...)
	}
	if len(alerts) == 0 {
		t.Fatal("期望报出 RTP 异常，实际没有")
	}
	if alerts[0].RtpPPM < 1800000 {
		t.Fatalf("RTP ppm = %d，应高于阈值", alerts[0].RtpPPM)
	}
}

// TestWatchdogIgnoresNormalVariance —— 这是本次修的回归点。
//
// 旧实现只需要 10 局就参与判定，而本游戏单局 σ≈6 倍投注额：
// 10 局里只要出现一次 20x，窗口 RTP 就冲到 200% 以上，被误判成"RTP 漂移"，
// 于是 flagged 游戏、封禁玩家、之后全部下注返回 403。
// 96% 的正常游戏不该因为一个 20x 就被封。
func TestWatchdogIgnoresNormalVariance(t *testing.T) {
	w := New(Config{GlobalMax: 10000, PlayerMax: 10000,
		ThresholdPPM: 1800000, MinSampleBet: 100 * money.Scale}) // MinSamples 走默认 2000

	// 10 局里中一个 20x：窗口 RTP = (9×0 + 200) / 100 = 200% > 180%
	bet := int64(10 * money.Scale)
	var alerts []Alert
	for i := 0; i < 10; i++ {
		win := int64(0)
		if i == 0 {
			win = 20 * money.Scale
		}
		alerts = append(alerts, w.Record(RecordInput{
			MerchantCode: "m001", GameCode: "fishing", UserID: "10001",
			BetMinor: bet, WinMinor: win,
		})...)
	}
	if len(alerts) != 0 {
		t.Fatalf("10 局的小样本不应触发告警（样本量不足），实际报出 %d 条：%+v", len(alerts), alerts)
	}
}

// TestDefaultConfigIsStatisticallySane —— 默认值必须自洽：
// 阈值 180% 配 2000 局样本时，6σ 上界仍低于阈值（详见 Config.MinSamples 的推导）。
func TestDefaultConfigIsStatisticallySane(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MinSamples < 2000 {
		t.Errorf("MinSamples = %d，小样本会让正常波动被误判为 RTP 漂移（需 ≥2000）", cfg.MinSamples)
	}
	if cfg.PlayerMax < cfg.MinSamples {
		t.Errorf("PlayerMax(%d) < MinSamples(%d)：玩家维度永远不会告警", cfg.PlayerMax, cfg.MinSamples)
	}
	if cfg.GlobalMax < cfg.MinSamples {
		t.Errorf("GlobalMax(%d) < MinSamples(%d)：游戏维度永远不会告警", cfg.GlobalMax, cfg.MinSamples)
	}
	if cfg.AlertCooldown <= 0 {
		t.Error("AlertCooldown 必须为正，否则窗口越线后会逐局刷屏告警")
	}
}

// TestWatchdogClampsWindowToMinSamples —— 窗口配小了会被自动抬高，
// 否则该维度静默失效（不报错、也不告警，最难排查）。
func TestWatchdogClampsWindowToMinSamples(t *testing.T) {
	w := New(Config{GlobalMax: 10, PlayerMax: 10, MinSamples: 500,
		ThresholdPPM: 1800000, MinSampleBet: money.Scale})
	if w.playerMax != 500 || w.globalMax != 500 {
		t.Fatalf("窗口未被抬到 MinSamples：playerMax=%d globalMax=%d", w.playerMax, w.globalMax)
	}
}

// TestRedisWatchdogCoolsDownAfterAlert —— 窗口越线后不得逐局重复告警。
// 实测旧实现会在 10 局后连续刷 risk_alerts + 反复 upsert 黑名单。
func TestRedisWatchdogCoolsDownAfterAlert(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	wd := NewRedis(rdb, Config{
		GlobalMax: 100, PlayerMax: 100, MinSamples: 10,
		ThresholdPPM: 1800000, MinSampleBet: money.Scale,
		AlertCooldown: time.Hour,
	})

	ctx := context.Background()
	countAlerts := func() int {
		alerts, err := wd.Record(ctx, RecordInput{
			MerchantCode: "m001", GameCode: "fishing", UserID: "10001",
			BetMinor: 10 * money.Scale, WinMinor: 20 * money.Scale,
		})
		if err != nil {
			t.Fatal(err)
		}
		return len(alerts)
	}

	total := 0
	for i := 0; i < 20; i++ {
		total += countAlerts()
	}
	// 第 10 局首次越线各报 1 条（user + game），之后冷却期内不再报
	if total == 0 {
		t.Fatal("越线后应当至少报出一次告警")
	}
	if total > 2 {
		t.Fatalf("冷却期内重复告警：20 局共报出 %d 条（期望仅首次的 2 条）", total)
	}

	// 冷却键过期后可以再次告警
	mr.FastForward(2 * time.Hour)
	if got := countAlerts(); got == 0 {
		t.Fatal("冷却期结束后应能重新告警")
	}
}

// TestRedisWatchdogRequiresMinSamples —— 样本量不足时不得告警（与内存版一致）。
func TestRedisWatchdogRequiresMinSamples(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	wd := NewRedis(rdb, Config{
		GlobalMax: 10000, PlayerMax: 10000, MinSamples: 2000,
		ThresholdPPM: 1800000, MinSampleBet: money.Scale,
	})

	ctx := context.Background()
	for i := 0; i < 50; i++ {
		alerts, err := wd.Record(ctx, RecordInput{
			MerchantCode: "m001", GameCode: "fishing", UserID: "10001",
			BetMinor: 10 * money.Scale, WinMinor: 50 * money.Scale, // 500% RTP
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(alerts) != 0 {
			t.Fatalf("仅 %d 局（< MinSamples 2000）不应告警", i+1)
		}
	}
}
