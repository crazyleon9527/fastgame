package prng

import (
	"sync"

	"fastgame/pkg/money"
	"fastgame/pkg/par"
)

type Outcome struct {
	Multiplier   money.Multiplier
	WinAmount    money.Amount
	FishState    string
	AnimationKey string
	RtpTier      string
	Roll         uint64

	// 命中的 PAR 行，供表现层与对账使用（客户端按 AnimationKey 播动画，
	// 这两个字段是给后台/回放页展示"打到了哪条鱼"用的）。
	FishID   int
	FishName string
	FishTier string
}

type FairProof struct {
	ServerSeedHash string
	ServerSeed     string
	ClientSeed     string
	Nonce          string
	Roll           uint64
}

type Engine struct {
	rtpTier string
	table   *par.Table
}

var enginePool = sync.Pool{
	New: func() any { return &Engine{rtpTier: "default", table: par.Default96} },
}

func NewEngine(rtpTier string) *Engine {
	e := enginePool.Get().(*Engine)
	if rtpTier == "" {
		rtpTier = "default"
	}
	e.rtpTier = rtpTier
	e.table = par.Default96
	return e
}

func (e *Engine) Release() {
	e.rtpTier = "default"
	e.table = par.Default96
	enginePool.Put(e)
}

// Table 返回当前使用的赔付表，便于监控/测试核对理论 RTP。
func (e *Engine) Table() *par.Table { return e.table }

func (e *Engine) Spin(serverSeed, clientSeed, nonce string, betAmount money.Amount) (Outcome, FairProof, error) {
	roll, err := RollUint64(serverSeed, clientSeed, nonce)
	if err != nil {
		return Outcome{}, FairProof{}, err
	}

	proof := FairProof{
		ServerSeedHash: HashServerSeed(serverSeed),
		ServerSeed:     serverSeed,
		ClientSeed:     clientSeed,
		Nonce:          nonce,
		Roll:           roll,
	}

	outcome := e.outcomeFromRoll(roll, betAmount)
	outcome.Roll = roll
	return outcome, proof, nil
}

// outcomeFromRoll 把 provably-fair 的 roll 映射成赔付结果。
//
// 这里是唯一的结算口径：/game/bet 与 /game/replay、/verify 都走它，
// 因此"回放算出来的钱和实际派彩不一致"这种偏差不可能出现。
//
// 结果完全由 roll 决定（roll 由 HMAC(serverSeed, clientSeed:nonce:0) 得出），
// 所以客户端可以用同一套算法独立复现——这也是 web/shared/prng-money.js 必须
// 镜像 pkg/par 表的原因。
func (e *Engine) outcomeFromRoll(roll uint64, betAmount money.Amount) Outcome {
	table := e.table
	if table == nil {
		table = par.Default96
	}
	hit := table.Sample(roll)

	multiplier := money.Multiplier(hit.MultiplierMinor)
	outcome := Outcome{
		Multiplier: multiplier,
		WinAmount:  multiplier.Apply(betAmount),
		RtpTier:    e.rtpTier,
		FishID:     hit.ID,
		FishName:   hit.Name,
		FishTier:   hit.Tier,
	}
	outcome.FishState, outcome.AnimationKey = animationFor(hit.Tier)
	return outcome
}

// animationFor 把 PAR 档位映射到客户端既有的三态动画。
//
// 保持 "miss" / "bite" / "big_win" 三态是为了不改 Cocos 客户端：
// 换赔付表不应该要求客户端同步发版。
func animationFor(tier string) (fishState, animationKey string) {
	switch tier {
	case par.TierMiss:
		return "miss", "fish_miss"
	case par.TierBoss:
		return "big_win", "fish_bite_bigwin"
	default:
		return "bite", "fish_bite_normal"
	}
}

// MaxPayoutMinor 返回当前表的最高赔付倍率（定点），供风控与上限校验使用。
func MaxPayoutMinor() int64 { return par.Default96.MaxMultiplierMinor() }

// TheoreticalRTP 返回当前表的理论 RTP（0.96 表示 96%）。
// 监控应当拿实际 RTP 与它比对，而不是写死 96——换表时告警阈值自动跟着走。
func TheoreticalRTP() float64 { return par.Default96.RTP() }
