package par

import (
	"math"
	"testing"
)

// TestDefault96RTPIsExact —— 这张表的存在意义就是 RTP 必须精确等于 96%。
// 用整数比校验而不是浮点近似：任何权重或倍率被改动都会立刻失败。
func TestDefault96RTPIsExact(t *testing.T) {
	var payout uint64
	for _, e := range Default96.Entries() {
		payout += uint64(e.MultiplierMinor) * uint64(e.Weight)
	}
	// RTP = payout / (totalWeight × Scale)，要求恰好等于 96/100：
	//   payout × 100 == 96 × totalWeight × Scale
	lhs := payout * 100
	rhs := uint64(96) * Default96.TotalWeight() * Scale
	if lhs != rhs {
		t.Fatalf("PAR 表已不满足 96%% RTP：Σ(权重×倍率) = %d，%d != %d", payout, lhs, rhs)
	}

	if rtp := Default96.RTP(); math.Abs(rtp-0.96) > 1e-12 {
		t.Fatalf("RTP() = %.17f，期望 0.96", rtp)
	}
	if tw := Default96.TotalWeight(); tw != 1_000_000 {
		t.Fatalf("总权重 = %d，期望 1000000", tw)
	}
}

// TestSampleCoversWholeRollRange —— 采样必须覆盖 [0, MaxUint64] 且不越界。
// 旧实现用 uint64 乘法，roll 一大就回绕，倍率分布被破坏。
func TestSampleCoversWholeRollRange(t *testing.T) {
	cases := []struct {
		roll    uint64
		wantID  int
		comment string
	}{
		{0, 0, "roll=0 落在第一档（空杆）"},
		{math.MaxUint64, 10, "roll 取最大也必须是最后一档，不能溢出回绕"},
		{math.MaxUint64 / 2, -1, "中位 roll 落在表内（档位不作断言）"},
	}
	for _, c := range cases {
		got := Default96.Sample(c.roll)
		if c.wantID >= 0 && got.ID != c.wantID {
			t.Errorf("roll=%d 命中 id=%d，期望 %d（%s）", c.roll, got.ID, c.wantID, c.comment)
		}
		if got.ID < 0 || got.ID > 10 {
			t.Errorf("roll=%d 命中越界 id=%d", c.roll, got.ID)
		}
	}
}

// TestSampleIsDeterministic —— 同一 roll 必须永远得到同一结果（可验证公平性的前提）。
func TestSampleIsDeterministic(t *testing.T) {
	for _, roll := range []uint64{1, 12345, 1 << 40, math.MaxUint64 - 1} {
		a := Default96.Sample(roll)
		b := Default96.Sample(roll)
		if a != b {
			t.Fatalf("roll=%d 两次采样结果不一致: %+v vs %+v", roll, a, b)
		}
	}
}

// TestSampleDistributionMatchesWeights —— 统计频率必须收敛到权重占比。
// 这是"表写对了"的端到端证据（而不是只看公式）。
func TestSampleDistributionMatchesWeights(t *testing.T) {
	const n = 2_000_000
	counts := make(map[int]int, len(Default96.Entries()))
	// 用步长制造均匀覆盖；乘一个奇数步长避免与权重产生规律性对齐
	for i := uint64(0); i < n; i++ {
		roll := i * 0x9E3779B97F4A7C15 // 黄金比例步长
		counts[Default96.Sample(roll).ID]++
	}

	for _, e := range Default96.Entries() {
		got := float64(counts[e.ID]) / n
		want := float64(e.Weight) / float64(Default96.TotalWeight())
		// 2e6 样本、最小权重 40/1e6（期望 80 次）→ 用相对误差 5%
		if want > 0 && math.Abs(got-want)/want > 0.05 {
			t.Errorf("%s(id=%d) 频率 %.5f，期望 %.5f（偏差 >5%%）", e.Name, e.ID, got, want)
		}
	}
}

// TestMaxMultiplier —— 风控上限依赖它。
func TestMaxMultiplier(t *testing.T) {
	if got := Default96.MaxMultiplierMinor(); got != 5_000_000 {
		t.Fatalf("最大倍率 = %d，期望 5000000（500x）", got)
	}
	if got := Default96.Entries()[9].Multiplier(); got != 250.0 {
		t.Fatalf("Giant Squid 倍率 = %v，期望 250", got)
	}
}
