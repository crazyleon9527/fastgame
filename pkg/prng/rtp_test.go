package prng

import (
	"fmt"
	"testing"

	"fastgame/pkg/money"
	"fastgame/pkg/par"
)

// TestOutcomeFollowsPARTable —— 结算结果必须逐局等于 PAR 表按 roll 采样的结果。
// 这条断言是「RGS 用 PAR 表结算」的直接证据，而不是看 RTP 凑不凑得上。
func TestOutcomeFollowsPARTable(t *testing.T) {
	e := NewEngine("default")
	defer e.Release()

	bet := money.FromMajor(1)
	allowed := map[int64]bool{}
	for _, entry := range par.Default96.Entries() {
		allowed[entry.MultiplierMinor] = true
	}

	checked := 0
	for i := 0; i < 5000; i++ {
		nonce := fmt.Sprintf("round-%d", i)
		roll, err := RollUint64("server-seed", "client-seed", nonce)
		if err != nil {
			t.Fatal(err)
		}
		hit := par.Default96.Sample(roll)
		out, _, err := e.Spin("server-seed", "client-seed", nonce, bet)
		if err != nil {
			t.Fatal(err)
		}

		if int64(out.Multiplier) != hit.MultiplierMinor {
			t.Fatalf("nonce=%s roll=%d：结算倍率 %d，PAR 表给出 %d —— 结算没有走 PAR 表",
				nonce, roll, int64(out.Multiplier), hit.MultiplierMinor)
		}
		if out.FishID != hit.ID || out.FishName != hit.Name {
			t.Fatalf("nonce=%s：命中信息 %d/%s，PAR 表给出 %d/%s",
				nonce, out.FishID, out.FishName, hit.ID, hit.Name)
		}
		if !allowed[int64(out.Multiplier)] {
			t.Fatalf("nonce=%s：倍率 %d 不在 PAR 表里（旧赔付表 1.5~5x / 50~100x 的连续区间说明没换表）",
				nonce, int64(out.Multiplier))
		}
		wantWin := money.Multiplier(hit.MultiplierMinor).Apply(bet)
		if out.WinAmount != wantWin {
			t.Fatalf("nonce=%s：派彩 %d，期望 %d", nonce, out.WinAmount.Minor(), wantWin.Minor())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("没有校验任何一局")
	}
}

// TestAnimationMappingStable —— 换赔付表不该要求客户端发版，
// 因此三态动画映射必须保持旧值。
func TestAnimationMappingStable(t *testing.T) {
	cases := []struct {
		tier      string
		fishState string
		animKey   string
	}{
		{par.TierMiss, "miss", "fish_miss"},
		{par.TierCommon, "bite", "fish_bite_normal"},
		{par.TierRare, "bite", "fish_bite_normal"},
		{par.TierBoss, "big_win", "fish_bite_bigwin"},
	}
	for _, c := range cases {
		gotState, gotKey := animationFor(c.tier)
		if gotState != c.fishState || gotKey != c.animKey {
			t.Errorf("档位 %s → (%s,%s)，期望 (%s,%s)", c.tier, gotState, gotKey, c.fishState, c.animKey)
		}
	}
}

// TestSettlementRTPConvergesTo96 —— 结算链路的 RTP 收敛。
//
// 这是本项目最严重缺陷的回归防线：旧赔付表（55% 空杆 / 35% 拿 1.5~5x /
// 10% 拿 50~100x）期望倍率 8.64，实测 RTP 1300%~1800%，会被 rtpwatchdog
// 判定漂移并把游戏置为 flagged，进而让全部下注返回 403。
//
// 容差按统计显著性设定：
//
//	单局派彩 E[X]=0.96、E[X²]≈37.36 → σ≈6.04（以 1 注为单位）
//	30 万局 → SE = 6.04/√300000 ≈ 1.1%，±5% 约 4.5σ
//
// 真错的话偏差是百分之几百，不可能落进这个区间。
func TestSettlementRTPConvergesTo96(t *testing.T) {
	e := NewEngine("default")
	defer e.Release()

	const rounds = 300_000
	bet := money.FromMajor(1)

	var totalBet, totalWin int64
	for i := 0; i < rounds; i++ {
		nonce := fmt.Sprintf("rtp-round-%d", i)
		out, _, err := e.Spin("rtp-server-seed", "rtp-client-seed", nonce, bet)
		if err != nil {
			t.Fatal(err)
		}
		totalBet += bet.Minor()
		totalWin += out.WinAmount.Minor()
	}

	actual := float64(totalWin) / float64(totalBet)
	theoretical := TheoreticalRTP()
	t.Logf("理论 RTP %.4f%%，%d 局实测 RTP %.4f%%", theoretical*100, rounds, actual*100)

	const tolerance = 0.05
	if diff := actual - theoretical; diff > tolerance || diff < -tolerance {
		t.Fatalf("结算 RTP 收敛失败：实测 %.4f%%，理论 %.4f%%（偏差 %.4f 超出 ±%.2f）",
			actual*100, theoretical*100, diff, tolerance)
	}
	if theoretical != 0.96 {
		t.Fatalf("理论 RTP = %v，期望 0.96 —— PAR 表被改动了", theoretical)
	}
}

// BenchmarkOutcomeFromRoll 采样必须在微秒级（下注主链路）。
func BenchmarkOutcomeFromRoll(b *testing.B) {
	e := NewEngine("default")
	defer e.Release()
	bet := money.FromMajor(1)
	rolls := []uint64{0, 1 << 20, 1 << 40, 1 << 63, ^uint64(0)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = e.outcomeFromRoll(rolls[i%len(rolls)], bet)
	}
}
