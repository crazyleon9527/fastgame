package prng

import (
	"math"
	"sync"

	"fastgame/pkg/money"
)

type Outcome struct {
	Multiplier   money.Multiplier
	WinAmount    money.Amount
	FishState    string
	AnimationKey string
	RtpTier      string
	Roll         uint64
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
}

var enginePool = sync.Pool{
	New: func() any { return &Engine{rtpTier: "default"} },
}

func NewEngine(rtpTier string) *Engine {
	e := enginePool.Get().(*Engine)
	if rtpTier == "" {
		rtpTier = "default"
	}
	e.rtpTier = rtpTier
	return e
}

func (e *Engine) Release() {
	e.rtpTier = "default"
	enginePool.Put(e)
}

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

	outcome := e.outcomeFromRoll(roll, serverSeed, clientSeed, nonce, betAmount)
	outcome.Roll = roll
	return outcome, proof, nil
}

func (e *Engine) outcomeFromRoll(roll uint64, serverSeed, clientSeed, nonce string, betAmount money.Amount) Outcome {
	switch {
	case rollBelow(roll, 55, 100):
		return Outcome{
			Multiplier:   0,
			WinAmount:    0,
			FishState:    "miss",
			AnimationKey: "fish_miss",
			RtpTier:      e.rtpTier,
		}
	case rollBelow(roll, 90, 100):
		multiRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 1)
		mult := multiplierFromRange(multiRoll, 15000, 50000) // 1.5x ~ 5.0x
		return Outcome{
			Multiplier:   mult,
			WinAmount:    mult.Apply(betAmount),
			FishState:    "bite",
			AnimationKey: "fish_bite_normal",
			RtpTier:      e.rtpTier,
		}
	default:
		multiRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 1)
		mult := multiplierFromRange(multiRoll, 500000, 1000000) // 50x ~ 100x
		return Outcome{
			Multiplier:   mult,
			WinAmount:    mult.Apply(betAmount),
			FishState:    "big_win",
			AnimationKey: "fish_bite_bigwin",
			RtpTier:      e.rtpTier,
		}
	}
}

// multiplierFromRange 线性映射 multiRoll ∈ [0,MaxUint64] → [minMult, maxMult]（纯整数）
func multiplierFromRange(multiRoll uint64, minMult, maxMult int64) money.Multiplier {
	if maxMult <= minMult {
		return money.Multiplier(minMult)
	}
	span := uint64(maxMult - minMult)
	return money.Multiplier(minMult + int64(uint64(multiRoll)*span/math.MaxUint64))
}
