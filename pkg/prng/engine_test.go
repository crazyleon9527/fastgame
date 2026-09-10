package prng

import (
	"fmt"
	"testing"

	"fastgame/pkg/money"
)

func TestSpinNeverNegativeWin(t *testing.T) {
	engine := NewEngine("high")
	defer engine.Release()
	bet := money.FromMajor(10)
	for i := 0; i < 200; i++ {
		out, _, err := engine.Spin("server-seed", "client-seed", fmt.Sprintf("round-%d", i), bet)
		if err != nil {
			t.Fatal(err)
		}
		if out.WinAmount < 0 || out.Multiplier < 0 {
			t.Fatalf("negative outcome: %+v", out)
		}
	}
}

func TestProvablyFairProof(t *testing.T) {
	engine := NewEngine("default")
	defer engine.Release()
	_, proof, err := engine.Spin("abc123", "client456", "round-1", money.FromMajor(10))
	if err != nil {
		t.Fatal(err)
	}
	if proof.ServerSeedHash != HashServerSeed("abc123") {
		t.Fatal("hash mismatch")
	}
	roll, _ := RollUint64("abc123", "client456", "round-1")
	if proof.Roll != roll {
		t.Fatalf("proof roll mismatch: %d vs %d", proof.Roll, roll)
	}
}

func BenchmarkSpinZeroAlloc(b *testing.B) {
	engine := NewEngine("default")
	bet := money.FromMajor(10)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out, _, err := engine.Spin("server-seed", "client-seed", "bench-round", bet)
		if err != nil || out.FishState == "" {
			b.Fatal(err)
		}
	}
	engine.Release()
}
