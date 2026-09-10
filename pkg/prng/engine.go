package prng

import (
	"crypto/rand"
	"encoding/binary"
	"math"
)

type Outcome struct {
	Multiplier   float64
	WinAmount    float64
	FishState    string
	AnimationKey string
	RtpTier      string
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

// Spin calculates outcome from bet amount using secure randomness.
func (e *Engine) Spin(betAmount float64) Outcome {
	roll := secureFloat()

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
		multiplier := 1.5 + secureFloat()*3.5
		return Outcome{
			Multiplier:   round2(multiplier),
			WinAmount:    round2(betAmount * multiplier),
			FishState:    "bite",
			AnimationKey: "fish_bite_normal",
			RtpTier:      e.rtpTier,
		}
	default:
		multiplier := 50 + secureFloat()*50
		return Outcome{
			Multiplier:   round2(multiplier),
			WinAmount:    round2(betAmount * multiplier),
			FishState:    "big_win",
			AnimationKey: "fish_bite_bigwin",
			RtpTier:      e.rtpTier,
		}
	}
}

func secureFloat() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	n := binary.LittleEndian.Uint64(b[:])
	return float64(n) / float64(math.MaxUint64)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
