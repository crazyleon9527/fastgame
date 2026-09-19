package fishing

import (
	"context"
	"encoding/json"
	"fmt"

	"fastgame/engine"
	"fastgame/pkg/money"
	"fastgame/pkg/par"
	"fastgame/pkg/prng"
)

const (
	GameCodeFishing = "fishing_tycoon"
	MathVersionV1   = "v1.0.0_tier96"
)

// FishingPlugin 钓鱼大亨数学推演插件
type FishingPlugin struct {
	table *par.Table
}

func init() {
	// 启动时自动完成预编译并注册进 Universal Host
	engine.RegisterPlugin(NewFishingPlugin(DefaultPARTable96))
}

func NewFishingPlugin(table []FishDef) *FishingPlugin {
	return &FishingPlugin{table: par.NewTable(table)}
}

func (p *FishingPlugin) GameCode() string {
	return GameCodeFishing
}

// CalculateOutcome 核心推演。
//
// 结果由 provably-fair 的 roll 决定：roll = HMAC-SHA256(serverSeed, clientSeed:roundId:0)
// 的高 64 位，再按 PAR 表权重区间定位命中项。这与 RGS 结算（pkg/prng）用的是
// 同一套算法和同一张表，因此 /verify 与 /replay 能独立复现派彩。
//
// 上一版这里用 crypto/rand 直接抽结果，并现场生成一对新种子——那样开奖结果与
// 返回给客户端的种子毫无关系，既无法回放也无法验证，属于必须修掉的缺陷。
func (p *FishingPlugin) CalculateOutcome(_ context.Context, in *engine.TurnInput) (*engine.TurnOutcome, error) {
	if in == nil {
		return nil, fmt.Errorf("nil turn input")
	}
	if in.ServerSeed == "" || in.ClientSeed == "" || in.RoundID == "" {
		return nil, fmt.Errorf("provably fair inputs required: server_seed / client_seed / round_id")
	}

	roll, err := prng.RollUint64(in.ServerSeed, in.ClientSeed, in.RoundID)
	if err != nil {
		return nil, fmt.Errorf("provably fair roll failed: %w", err)
	}

	hit := p.table.Sample(roll)
	winAmount := money.Multiplier(hit.MultiplierMinor).Apply(in.BetAmount)

	payloadJSON, _ := json.Marshal(map[string]any{
		"caught":         hit.ID > 0,
		"fish_id":        hit.ID,
		"fish_name":      hit.Name,
		"tier":           hit.Tier,
		"multiplier":     hit.Multiplier(),
		"tension_ms":     hit.TensionMs,
		"is_big_win":     hit.MultiplierMinor >= 50*par.Scale,
		"coin_drop_tier": getCoinDropTier(hit.Multiplier()),
	})

	return &engine.TurnOutcome{
		WinAmount:           winAmount,
		PayoutMultiplier:    hit.Multiplier(),
		MathVersion:         MathVersionV1,
		RtpApplied:          p.table.RTP() * 100,
		ServerSeed:          in.ServerSeed,
		ServerSeedHash:      prng.HashServerSeed(in.ServerSeed),
		ClientSeed:          in.ClientSeed,
		Nonce:               0, // roll 索引：0 即主开奖 roll
		PresentationPayload: string(payloadJSON),
	}, nil
}

// TableRTP 暴露本插件表的理论 RTP（百分比），供监控与自检使用。
func (p *FishingPlugin) TableRTP() float64 { return p.table.RTP() * 100 }

func getCoinDropTier(mult float64) string {
	switch {
	case mult >= 100.0:
		return "FOUNTAIN" // 爆裂金币喷泉
	case mult >= 20.0:
		return "BURST" // 大量金币迸发
	case mult > 0.0:
		return "NORMAL" // 普通金币收集
	default:
		return "NONE"
	}
}
