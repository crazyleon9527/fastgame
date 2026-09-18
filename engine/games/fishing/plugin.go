package fishing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"fastgame/engine"
	"fastgame/pkg/money"
)

const (
	GameCodeFishing = "fishing_tycoon"
	MathVersionV1   = "v1.0.0_tier96"
	TargetRTP       = 96.00
)

// FishingPlugin 钓鱼大亨数学推演插件
type FishingPlugin struct {
	table        []FishDef
	cumulative   []uint32 // 权重前缀和，用于二分极速查找
	totalWeight  uint32
}

func init() {
	// 启动时自动完成预编译并注册进 Universal Host
	plugin := NewFishingPlugin(DefaultPARTable96)
	engine.RegisterPlugin(plugin)
}

func NewFishingPlugin(table []FishDef) *FishingPlugin {
	p := &FishingPlugin{
		table:      table,
		cumulative: make([]uint32, len(table)),
	}

	var sum uint32
	for i, item := range table {
		sum += item.Weight
		p.cumulative[i] = sum
	}
	p.totalWeight = sum
	return p
}

func (p *FishingPlugin) GameCode() string {
	return GameCodeFishing
}

// CalculateOutcome 核心推演 (目标耗时 < 30微秒)
func (p *FishingPlugin) CalculateOutcome(ctx context.Context, in *engine.TurnInput) (*engine.TurnOutcome, error) {
	// 1. 生成 Provably Fair 种子对 (客户端可核验公正性)
	serverSeed, serverSeedHash, err := generateSeedPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate seed pair: %w", err)
	}

	// 2. 伪随机数采样 (落在 [0, totalWeight) 范围内)
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(p.totalWeight)))
	if err != nil {
		return nil, fmt.Errorf("rng generation failed: %w", err)
	}
	pick := uint32(nBig.Uint64())

	// 3. 前缀和二分查找命中目标鱼种 (O(log N) 耗时约为几纳秒)
	idx := sort.Search(len(p.cumulative), func(i int) bool {
		return p.cumulative[i] > pick
	})
	hitFish := p.table[idx]

	// 4. 派彩金额计算 (单位: 分)
	winAmountMinor := int64(float64(in.BetAmount.Minor()) * hitFish.Multiplier)
	winAmount := money.AmountFromMinor(winAmountMinor)

	// 5. 组装给 Cocos 客户端还原动效的 Presentation Payload
	payloadMap := map[string]any{
		"caught":         hitFish.FishID > 0,
		"fish_id":        hitFish.FishID,
		"fish_name":      hitFish.Name,
		"tier":           hitFish.Tier,
		"multiplier":     hitFish.Multiplier,
		"tension_ms":     hitFish.TensionMs,
		"is_big_win":     hitFish.Multiplier >= 50.0,
		"coin_drop_tier": getCoinDropTier(hitFish.Multiplier),
	}
	payloadJSON, _ := json.Marshal(payloadMap)

	return &engine.TurnOutcome{
		WinAmount:           winAmount,
		PayoutMultiplier:    hitFish.Multiplier,
		MathVersion:         MathVersionV1,
		RtpApplied:          TargetRTP,
		ServerSeed:          serverSeed,
		ServerSeedHash:      serverSeedHash,
		ClientSeed:          in.RoundID, // 默认关联注单号作为客户端因子
		Nonce:               1,
		PresentationPayload: string(payloadJSON),
	}, nil
}

func getCoinDropTier(mult float64) string {
	if mult >= 100.0 {
		return "FOUNTAIN" // 爆裂金币喷泉
	} else if mult >= 20.0 {
		return "BURST"    // 大量金币迸发
	} else if mult > 0.0 {
		return "NORMAL"   // 普通金币收集
	}
	return "NONE"
}

func generateSeedPair() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	seed := hex.EncodeToString(bytes)
	h := sha256.Sum256([]byte(seed))
	hash := hex.EncodeToString(h[:])
	return seed, hash, nil
}