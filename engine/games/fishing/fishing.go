package fishing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"fastgame/engine"
	"fastgame/pkg/money"
	"fastgame/pkg/par"
	"fastgame/pkg/prng"
)

const (
	GameCodeFishing = "fishing_tycoon"
)

// PresentationPayload 前端视觉演播载荷 (强类型，零 map 装箱)
type PresentationPayload struct {
	Caught       bool    `json:"caught"`
	FishID       int     `json:"fish_id"`
	FishName     string  `json:"fish_name"`
	Tier         string  `json:"tier"`
	Multiplier   float64 `json:"multiplier"`
	TensionMs    int     `json:"tension_ms"`
	IsBigWin     bool    `json:"is_big_win"`
	CoinDropTier string  `json:"coin_drop_tier"`
}

type FishingPlugin struct {
	tables map[string]*par.Table // 档位映射: "96" -> Table96, "94" -> Table94
}

func init() {
	plugin := NewFishingPlugin()
	engine.RegisterPlugin(plugin)
}

func NewFishingPlugin() *FishingPlugin {
	tables := make(map[string]*par.Table)
	tables["96"] = par.Default96
	tables["94"] = par.Default94

	return &FishingPlugin{
		tables: tables,
	}
}

func (p *FishingPlugin) GameCode() string {
	return GameCodeFishing
}

// CalculateOutcome 核心推演
func (p *FishingPlugin) CalculateOutcome(_ context.Context, in *engine.TurnInput) (*engine.TurnOutcome, error) {
	if in == nil {
		return nil, fmt.Errorf("nil turn input")
	}
	if in.ServerSeed == "" || in.ClientSeed == "" || in.RoundID == "" {
		return nil, fmt.Errorf("provably fair inputs required: server_seed / client_seed / round_id")
	}

	// 1. 动态选择目标 RTP 档位表
	tier := in.RtpTier
	if tier == "" {
		tier = "96"
	}
	table, ok := p.tables[tier]
	if !ok {
		table = p.tables["96"]
		tier = "96"
	}

	// 2. 可验证公平性采样
	roll, err := prng.RollUint64(in.ServerSeed, in.ClientSeed, in.RoundID)
	if err != nil {
		return nil, fmt.Errorf("provably fair roll failed: %w", err)
	}

	hit := table.Sample(roll)
	winAmount := money.Multiplier(hit.MultiplierMinor).Apply(in.BetAmount)

	// 3. 强类型极速序列化，消除反射 map[string]any
	presentation := PresentationPayload{
		Caught:       hit.ID > 0,
		FishID:       hit.ID,
		FishName:     hit.Name,
		Tier:         hit.Tier,
		Multiplier:   hit.Multiplier(),
		TensionMs:    hit.TensionMs,
		IsBigWin:     hit.MultiplierMinor >= 50*par.Scale,
		CoinDropTier: getCoinDropTier(hit.Multiplier()),
	}
	payloadBytes, _ := json.Marshal(presentation)

	return &engine.TurnOutcome{
		WinAmount:           winAmount,
		PayoutMultiplier:    hit.Multiplier(),
		MathVersion:         "v1.0.0_tier" + tier,
		RtpApplied:          table.RTP() * 100,
		ServerSeed:          in.ServerSeed,
		ServerSeedHash:      prng.HashServerSeed(in.ServerSeed),
		ClientSeed:          in.ClientSeed,
		Nonce:               0,
		PresentationPayload: string(payloadBytes),
	}, nil
}

func (p *FishingPlugin) TableRTP(tier string) float64 {
	if tbl, ok := p.tables[tier]; ok {
		return tbl.RTP() * 100
	}
	return p.tables["96"].RTP() * 100
}

func getCoinDropTier(mult float64) string {
	switch {
	case mult >= 100.0:
		return "FOUNTAIN"
	case mult >= 20.0:
		return "BURST"
	case mult > 0.0:
		return "NORMAL"
	default:
		return "NONE"
	}
}

// FloatToString 辅助无反射高效格式化
func FloatToString(val float64) string {
	return strconv.FormatFloat(val, 'f', 4, 64)
}
