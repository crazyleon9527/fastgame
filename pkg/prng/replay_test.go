package prng

import "testing"

func TestComputeReplayDeterministic(t *testing.T) {
	engine := NewEngine("default")
	a, _, err := engine.ComputeReplay("srv", "cli", "round-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := engine.ComputeReplay("srv", "cli", "round-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if a.Weather != b.Weather || a.FishSpecies != b.FishSpecies || a.BiteProp != b.BiteProp {
		t.Fatal("scene must be deterministic for same inputs")
	}
	if len(a.FishPath) != 3 || a.FishPath[0].X != b.FishPath[0].X {
		t.Fatal("fish path must match")
	}
	if a.Outcome.FishState != b.Outcome.FishState {
		t.Fatal("outcome must match")
	}
}

func TestComputeReplayDifferentNonce(t *testing.T) {
	engine := NewEngine("default")
	a, _, _ := engine.ComputeReplay("srv", "cli", "round-1", 10)
	b, _, _ := engine.ComputeReplay("srv", "cli", "round-2", 10)
	if a.Weather == b.Weather && a.FishPath[0].X == b.FishPath[0].X && a.Outcome.Roll == b.Outcome.Roll {
		t.Fatal("different nonce should produce different scene")
	}
}
