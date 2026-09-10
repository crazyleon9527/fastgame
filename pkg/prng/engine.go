package prng

import (
	"math"
)

type Outcome struct {
	Multiplier   float64
	WinAmount    float64
	FishState    string
	AnimationKey string
	RtpTier      string
	Roll         float64
}

type FairProof struct {
	ServerSeedHash string
	ServerSeed     string
	ClientSeed     string
	Nonce          string
	Roll           float64
}

type Engine struct {
	rtpTier string
}

func NewEngine(rtpTier string) *Engine {
	if rtpTier == "" {
		rtpTier = "default"
	}
	return &Engine{rtpTier: rtpTier}
}

// Spin computes outcome using provably fair HMAC-SHA256 roll (crypto/rand seeded server seed).
func (e *Engine) Spin(serverSeed, clientSeed, nonce string, betAmount float64) (Outcome, FairProof, error) {
	roll, err := Roll(serverSeed, clientSeed, nonce)
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

func (e *Engine) outcomeFromRoll(roll float64, serverSeed, clientSeed, nonce string, betAmount float64) Outcome {
	switch {
	case roll < 0.55:
		return Outcome{
			Multiplier:   0,
			WinAmount:    0,
			FishState:    "miss",
			AnimationKey: "fish_miss",
			RtpTier:      e.rtpTier,
		}
	case roll < 0.90:
		multiRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 1)
		multiplier := 1.5 + multiRoll*3.5
		return Outcome{
			Multiplier:   round2(multiplier),
			WinAmount:    round2(betAmount * multiplier),
			FishState:    "bite",
			AnimationKey: "fish_bite_normal",
			RtpTier:      e.rtpTier,
		}
	default:
		multiRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 1)
		multiplier := 50 + multiRoll*50
		return Outcome{
			Multiplier:   round2(multiplier),
			WinAmount:    round2(betAmount * multiplier),
			FishState:    "big_win",
			AnimationKey: "fish_bite_bigwin",
			RtpTier:      e.rtpTier,
		}
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
