// Package par 存放游戏的 PAR（Probability × Amount × Return）赔付表与确定性采样。
//
// 为什么单独成包：这张表同时被三处使用，必须是唯一事实来源
//   - pkg/prng           ：RGS 结算用它把 provably-fair 的 roll 映射成赔付
//   - engine/games/fishing：数学插件用它做推演与 RTP 自检
//   - web/shared/prng-money.js：客户端验算脚本镜像同一张表（改表必须同步改 JS）
//
// 倍率一律用 money 的定点口径（scale=10000）以 int64 存，避免浮点误差：
// 0.5x => 5000，500x => 5000000。weights 用整数权重，理论 RTP =
// Σ(weight × multiplierMinor) / (totalWeight × 10000)。
package par

import (
	"math/bits"
	"sort"
)

// Scale 与 pkg/money.Scale 一致：倍率的定点小数位。
const Scale = 10000

// Tier 命中档位，决定表现层动画（客户端按 Tier 选动画，不解析倍率）。
const (
	TierMiss   = "MISS"
	TierCommon = "COMMON"
	TierRare   = "RARE"
	TierBoss   = "BOSS"
)

// Entry 是赔付表的一行。
type Entry struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Tier            string `json:"tier"`
	MultiplierMinor int64  `json:"multiplier_minor"` // 倍率 ×10000，整数
	Weight          uint32 `json:"weight"`
	TensionMs       int    `json:"tension_ms"`
}

// Multiplier 返回浮点倍率，仅用于展示与日志。
func (e Entry) Multiplier() float64 {
	return float64(e.MultiplierMinor) / Scale
}

// Table 是预编译好的赔付表（权重前缀和，采样 O(log N)）。
type Table struct {
	entries    []Entry
	cumulative []uint64
	total      uint64
}

// NewTable 预编译一张赔付表。空表或全零权重会 panic——属于启动期配置错误，
// 与其在开奖时算出个 0 倍率，不如直接拒绝启动。
func NewTable(entries []Entry) *Table {
	if len(entries) == 0 {
		panic("par: empty payout table")
	}
	t := &Table{
		entries:    entries,
		cumulative: make([]uint64, len(entries)),
	}
	var sum uint64
	for i, e := range entries {
		if e.MultiplierMinor < 0 {
			panic("par: negative multiplier in payout table")
		}
		sum += uint64(e.Weight)
		t.cumulative[i] = sum
	}
	if sum == 0 {
		panic("par: payout table total weight is zero")
	}
	t.total = sum
	return t
}

// TotalWeight 返回权重总和。
func (t *Table) TotalWeight() uint64 { return t.total }

// Entries 返回表内容（只读，调用方不得修改）。
func (t *Table) Entries() []Entry { return t.entries }

// SamplePick 按 [0, totalWeight) 的整数采样返回命中行。
func (t *Table) SamplePick(pick uint64) Entry {
	i := sort.Search(len(t.cumulative), func(i int) bool {
		return t.cumulative[i] > pick
	})
	if i >= len(t.entries) {
		i = len(t.entries) - 1
	}
	return t.entries[i]
}

// Sample 把均匀分布在 [0, MaxUint64] 的 provably-fair roll 映射到权重区间。
//
// 用 bits.Mul64 取 128 位乘积的高 64 位，等价于 floor(roll × total / 2^64)，
// 结果精确落在 [0, total) 且无溢出——旧实现用 uint64 乘法，roll 稍大就回绕，
// 倍率会变成随机的（可验证性因此失效）。
func (t *Table) Sample(roll uint64) Entry {
	hi, _ := bits.Mul64(roll, t.total)
	return t.SamplePick(hi)
}

// RTP 返回理论 RTP（如 0.96 表示 96%）。
func (t *Table) RTP() float64 {
	var payout uint64
	for _, e := range t.entries {
		payout += uint64(e.MultiplierMinor) * uint64(e.Weight)
	}
	return float64(payout) / (float64(t.total) * Scale)
}

// MaxMultiplierMinor 返回表中的最大倍率，用于风控上限校验。
func (t *Table) MaxMultiplierMinor() int64 {
	var max int64
	for _, e := range t.entries {
		if e.MultiplierMinor > max {
			max = e.MultiplierMinor
		}
	}
	return max
}

// Default96 是钓鱼大亨默认赔付表：理论 RTP = 96.0000%。
//
//	总权重 1,000,000；Σ(权重 × 倍率定点) = 9,600,000,000；分母 1,000,000 × 10000
//	=> RTP = 0.96 精确成立（不是"接近"，是整数比）。
//
// 旧表（55% 空杆 / 35% 拿 1.5–5x / 10% 拿 50–100x）期望倍率 8.64，即 864% RTP，
// 会让 rtpwatchdog 判定 RTP 漂移并把游戏置为 flagged，导致全部下注被拒。
var Default96 = NewTable([]Entry{
	{ID: 0, Name: "Missed", Tier: TierMiss, MultiplierMinor: 0, Weight: 570000, TensionMs: 300},
	{ID: 1, Name: "Clownfish", Tier: TierCommon, MultiplierMinor: 5000, Weight: 160000, TensionMs: 600},
	{ID: 2, Name: "Sardine", Tier: TierCommon, MultiplierMinor: 10000, Weight: 170000, TensionMs: 700},
	{ID: 3, Name: "Flying Fish", Tier: TierCommon, MultiplierMinor: 20000, Weight: 50000, TensionMs: 800},
	{ID: 4, Name: "Tuna", Tier: TierRare, MultiplierMinor: 50000, Weight: 30000, TensionMs: 1200},
	{ID: 5, Name: "Manta Ray", Tier: TierRare, MultiplierMinor: 100000, Weight: 12000, TensionMs: 1500},
	{ID: 6, Name: "Swordfish", Tier: TierRare, MultiplierMinor: 200000, Weight: 5000, TensionMs: 1800},
	{ID: 7, Name: "Golden Turtle", Tier: TierBoss, MultiplierMinor: 500000, Weight: 2000, TensionMs: 2500},
	{ID: 8, Name: "Hammerhead Shark", Tier: TierBoss, MultiplierMinor: 1000000, Weight: 800, TensionMs: 3000},
	{ID: 9, Name: "Giant Squid", Tier: TierBoss, MultiplierMinor: 2500000, Weight: 160, TensionMs: 3500},
	{ID: 10, Name: "Megalodon", Tier: TierBoss, MultiplierMinor: 5000000, Weight: 40, TensionMs: 4500},
})
