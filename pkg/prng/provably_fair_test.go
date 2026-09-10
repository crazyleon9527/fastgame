package prng

import "testing"

func TestProvablyFairDeterministic(t *testing.T) {
	a, err := Roll("server-seed", "client-seed", "round-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Roll("server-seed", "client-seed", "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("expected deterministic roll, got %f vs %f", a, b)
	}
	if a < 0 || a >= 1 {
		t.Fatalf("roll out of range: %f", a)
	}
}

func TestProvablyFairDifferentNonce(t *testing.T) {
	a, _ := Roll("server-seed", "client-seed", "round-1")
	b, _ := Roll("server-seed", "client-seed", "round-2")
	if a == b {
		t.Fatal("different nonce should produce different roll")
	}
}
