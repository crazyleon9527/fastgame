package prng

import (
	"fmt"
	"testing"
)

func TestSpinNeverNegativeWin(t *testing.T) {
	engine := NewEngine("high")
	for i := 0; i < 200; i++ {
		out, _, err := engine.Spin("server-seed", "client-seed", fmt.Sprintf("round-%d", i), 10)
		if err != nil {
			t.Fatal(err)
		}
		if out.WinAmount < 0 || out.Multiplier < 0 {
			t.Fatalf("negative outcome: %+v", out)
		}
		if out.Roll < 0 || out.Roll >= 1 {
			t.Fatalf("roll out of range: %f", out.Roll)
		}
	}
}

func TestProvablyFairProof(t *testing.T) {
	engine := NewEngine("default")
	_, proof, err := engine.Spin("abc123", "client456", "round-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if proof.ServerSeedHash != HashServerSeed("abc123") {
		t.Fatal("hash mismatch")
	}
	roll, _ := Roll("abc123", "client456", "round-1")
	if proof.Roll != roll {
		t.Fatalf("proof roll mismatch: %f vs %f", proof.Roll, roll)
	}
}
