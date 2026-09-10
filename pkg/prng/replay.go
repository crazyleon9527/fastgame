package prng

import "math"

// ReplayInputs — 持久化最小集：种子 + 基础输入即可 100% 复现整局
type ReplayInputs struct {
	ServerSeed string  `json:"serverSeed"`
	ClientSeed string  `json:"clientSeed"`
	Nonce      string  `json:"nonce"`
	BetAmount  float64 `json:"betAmount"`
}

type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ReplayScene — 由种子确定性派生的完整场景（天气、轨迹、道具、派彩）
type ReplayScene struct {
	Weather        string    `json:"weather"`
	FishSpecies    string    `json:"fishSpecies"`
	FishPath       []Point2D `json:"fishPath"`
	BiteProp       string    `json:"biteProp"`
	CastDurationMs int       `json:"castDurationMs"`
	FishSpeed      float64   `json:"fishSpeed"`
	Outcome        Outcome   `json:"outcome"`
}

var (
	weatherOptions = []string{"clear", "cloudy", "rain", "storm"}
	speciesOptions = []string{"bass", "trout", "tuna", "salmon", "shark", "marlin"}
	bitePropOptions = []string{"worm", "lure", "fly", "jig"}
)

// ComputeReplay derives the full deterministic scene from seeds and bet amount.
func (e *Engine) ComputeReplay(serverSeed, clientSeed, nonce string, betAmount float64) (ReplayScene, FairProof, error) {
	roll, err := Roll(serverSeed, clientSeed, nonce)
	if err != nil {
		return ReplayScene{}, FairProof{}, err
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

	weatherRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 2)
	speciesRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 3)
	biteRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 10)
	castRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 11)
	speedRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 12)

	path := make([]Point2D, 3)
	for i := 0; i < 3; i++ {
		xRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 4+i*2)
		yRoll, _ := RollIndex(serverSeed, clientSeed, nonce, 5+i*2)
		path[i] = Point2D{
			X: round2(xRoll),
			Y: round2(yRoll),
		}
	}

	scene := ReplayScene{
		Weather:        weatherOptions[pickIndex(weatherRoll, len(weatherOptions))],
		FishSpecies:    speciesOptions[pickIndex(speciesRoll, len(speciesOptions))],
		FishPath:       path,
		BiteProp:       bitePropOptions[pickIndex(biteRoll, len(bitePropOptions))],
		CastDurationMs: 600 + int(castRoll*400),
		FishSpeed:      round2(0.5 + speedRoll*1.5),
		Outcome:        outcome,
	}
	return scene, proof, nil
}

func pickIndex(r float64, n int) int {
	if n <= 0 {
		return 0
	}
	idx := int(math.Floor(r * float64(n)))
	if idx >= n {
		return n - 1
	}
	if idx < 0 {
		return 0
	}
	return idx
}
