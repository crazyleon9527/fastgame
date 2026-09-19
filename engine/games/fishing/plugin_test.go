package fishing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fastgame/engine"
	"fastgame/pkg/money"
	"fastgame/pkg/par"
	"fastgame/pkg/prng"
)

// TestFishingRTPConvergence 蒙特卡洛校验赔付表的 RTP 收敛。
//
// 现在推演是确定性的（roll 由 provably-fair 种子派生），所以这里既验证 RTP，
// 也顺带验证"同一输入永远得到同一结果"——上一版用 crypto/rand 抽结果，
// 这条断言根本写不出来。
func TestFishingRTPConvergence(t *testing.T) {
	plugin := NewFishingPlugin(DefaultPARTable96)
	ctx := context.Background()

	const iterations = 1_000_000
	bet := money.FromMajor(1)

	var totalBet, totalWin int64
	start := time.Now()
	for i := 0; i < iterations; i++ {
		in := &engine.TurnInput{
			RoundID:    fmt.Sprintf("round-%d", i),
			ServerSeed: "sim-server-seed",
			ClientSeed: "sim-client-seed",
			BetAmount:  bet,
		}
		outcome, err := plugin.CalculateOutcome(ctx, in)
		if err != nil {
			t.Fatalf("推演出错: %v", err)
		}
		if i == 0 {
			again, err := plugin.CalculateOutcome(ctx, in)
			if err != nil {
				t.Fatal(err)
			}
			if again.WinAmount != outcome.WinAmount || again.PayoutMultiplier != outcome.PayoutMultiplier {
				t.Fatalf("同一输入两次推演结果不同：%+v vs %+v", outcome, again)
			}
			if outcome.ServerSeedHash != prng.HashServerSeed(in.ServerSeed) {
				t.Fatal("server_seed_hash 与 server_seed 不匹配")
			}
		}
		totalBet += bet.Minor()
		totalWin += outcome.WinAmount.Minor()
	}
	elapsed := time.Since(start)

	actualRTP := float64(totalWin) / float64(totalBet) * 100
	avgLatencyUs := float64(elapsed.Microseconds()) / float64(iterations)
	t.Logf("目标 RTP: %.4f%% | %d 局实测: %.4f%% | 单次平均 %.2f 微秒",
		TargetRTP, iterations, actualRTP, avgLatencyUs)

	// 容差按统计显著度设定：
	//   单局派彩 σ ≈ 6.04（0.5x/1x/2x/5x/... 的方差贡献主要在 100x~500x 尾巴）
	//   100 万局 → SE = 6.04/1000 ≈ 0.60%，±2% 约 3.3σ
	// 真错的话偏差是百分之几百（旧表是 864%），不可能落进这个区间。
	const tolerance = 2.0
	if diff := actualRTP - TargetRTP; diff < -tolerance || diff > tolerance {
		t.Errorf("❌ RTP 收敛偏差过大: %.4f%% (超出 ±%.1f%% 允许边界)", diff, tolerance)
	}

	if avgLatencyUs > 50.0 {
		t.Errorf("❌ 运算耗时超标: %.2f 微秒 > 50 微秒", avgLatencyUs)
	}
}

// TestFishingPluginRejectsMissingSeeds —— 缺 provably-fair 输入时必须报错。
// 静默退回随机数会让"可验证公平"变成一句空话。
func TestFishingPluginRejectsMissingSeeds(t *testing.T) {
	plugin := NewFishingPlugin(DefaultPARTable96)
	if _, err := plugin.CalculateOutcome(context.Background(), &engine.TurnInput{
		RoundID: "r1", BetAmount: money.FromMajor(1),
	}); err == nil {
		t.Fatal("缺少 server_seed/client_seed 时应报错")
	}
}

// TestFishingPluginUsesPARTable —— 插件结果必须等于 PAR 表按 roll 的采样，
// 且与 pkg/prng（RGS 结算口径）完全一致。
func TestFishingPluginUsesPARTable(t *testing.T) {
	plugin := NewFishingPlugin(DefaultPARTable96)
	bet := money.FromMajor(1)

	for i := 0; i < 2000; i++ {
		roundID := fmt.Sprintf("parity-%d", i)
		roll, err := prng.RollUint64("srv", "cli", roundID)
		if err != nil {
			t.Fatal(err)
		}
		hit := par.Default96.Sample(roll)

		out, err := plugin.CalculateOutcome(context.Background(), &engine.TurnInput{
			RoundID: roundID, ServerSeed: "srv", ClientSeed: "cli", BetAmount: bet,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.PayoutMultiplier != hit.Multiplier() {
			t.Fatalf("roundID=%s：插件倍率 %v，PAR 表 %v", roundID, out.PayoutMultiplier, hit.Multiplier())
		}

		// 与结算口径对齐
		e := prng.NewEngine("default")
		settle, _, err := e.Spin("srv", "cli", roundID, bet)
		e.Release()
		if err != nil {
			t.Fatal(err)
		}
		if settle.WinAmount != out.WinAmount {
			t.Fatalf("roundID=%s：结算派彩 %d，插件派彩 %d —— 两条链路口径不一致",
				roundID, settle.WinAmount.Minor(), out.WinAmount.Minor())
		}
	}
}
