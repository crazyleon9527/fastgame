package fishing

import (
	"context"
	"testing"
	"time"

	"fastgame/engine"
	"fastgame/pkg/money"
)

func TestFishingRTPConvergence(t *testing.T) {
	plugin := NewFishingPlugin(DefaultPARTable96)
	ctx := context.Background()

	const iterations = 10_000_000    // 模拟 1,000 万次下注
	betPerSpin := money.FromMajor(1) // 每注 $1.00 (100分)

	var totalBet int64
	var totalWin int64

	t.Logf("🚀 开始执行 %d 次蒙特卡洛模拟压测...", iterations)
	start := time.Now()

	in := &engine.TurnInput{
		RoundID:   "test_round_sim",
		BetAmount: betPerSpin,
	}

	for i := 0; i < iterations; i++ {
		outcome, err := plugin.CalculateOutcome(ctx, in)
		if err != nil {
			t.Fatalf("推演出错: %v", err)
		}
		totalBet += betPerSpin.Minor()
		totalWin += outcome.WinAmount.Minor()
	}

	elapsed := time.Since(start)
	avgLatencyUs := float64(elapsed.Microseconds()) / float64(iterations)
	actualRTP := (float64(totalWin) / float64(totalBet)) * 100

	t.Logf("✅ 压测完成! 总耗时: %s (单次平均耗时: %.2f 微秒)", elapsed, avgLatencyUs)
	t.Logf("🎯 目标 RTP: %.2f%% | 实际测出 RTP: %.4f%%", TargetRTP, actualRTP)

	// 1000 万次下，实际 RTP 与理论偏差必须在 ±0.15% 以内
	diff := actualRTP - TargetRTP
	if diff < -0.15 || diff > 0.15 {
		t.Errorf("❌ RTP 收敛偏差过大: %.4f%% (超出允许边界)", diff)
	} else {
		t.Logf("🎉 数学模型验证合格! 偏差仅为: %.4f%%", diff)
	}

	if avgLatencyUs > 50.0 {
		t.Errorf("❌ 运算耗时超标: %.2f 微秒 > 50 微秒", avgLatencyUs)
	}
}
