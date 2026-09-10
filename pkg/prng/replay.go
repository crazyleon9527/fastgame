package prng

import (
	"math"
	"sync"

	"fastgame/pkg/money"
)

type ReplayInputs struct {
	ServerSeed string       `json:"serverSeed"`
	ClientSeed string       `json:"clientSeed"`
	Nonce      string       `json:"nonce"`
	BetAmount  money.Amount `json:"betAmount"`
}

type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

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
	weatherOptions  = []string{"clear", "cloudy", "rain", "storm"}
	speciesOptions  = []string{"bass", "trout", "tuna", "salmon", "shark", "marlin"}
	bitePropOptions = []string{"worm", "lure", "fly", "jig"}
)

type replayScratch struct {
	path [3]Point2D
}

var replayPool = sync.Pool{
	New: func() any { return &replayScratch{} },
}

func (e *Engine) ComputeReplay(serverSeed, clientSeed, nonce string, betAmount money.Amount) (ReplayScene, FairProof, error) {
	roll, err := RollUint64(serverSeed, clientSeed, nonce)
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

	weatherRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 2)
	speciesRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 3)
	biteRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 10)
	castRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 11)
	speedRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 12)

	scratch := replayPool.Get().(*replayScratch)
	defer replayPool.Put(scratch)

	for i := 0; i < 3; i++ {
		xRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 4+i*2)
		yRoll, _ := RollIndexUint64(serverSeed, clientSeed, nonce, 5+i*2)
		scratch.path[i] = Point2D{
			X: coordFromRoll(xRoll),
			Y: coordFromRoll(yRoll),
		}
	}

	scene := ReplayScene{
		Weather:        weatherOptions[pickIndexUint64(weatherRoll, len(weatherOptions))],
		FishSpecies:    speciesOptions[pickIndexUint64(speciesRoll, len(speciesOptions))],
		FishPath:       scratch.path[:],
		BiteProp:       bitePropOptions[pickIndexUint64(biteRoll, len(bitePropOptions))],
		CastDurationMs: 600 + int(castRoll/(math.MaxUint64/400)),
		FishSpeed:      coordFromRoll(speedRoll)*1.5 + 0.5,
		Outcome:        outcome,
	}
	return scene, proof, nil
}

func pickIndexUint64(r uint64, n int) int {
	if n <= 0 {
		return 0
	}
	idx := int(r / (math.MaxUint64 / uint64(n)))
	if idx >= n {
		return n - 1
	}
	if idx < 0 {
		return 0
	}
	return idx
}

func coordFromRoll(r uint64) float64 {
	return float64(r) / float64(math.MaxUint64)
}
