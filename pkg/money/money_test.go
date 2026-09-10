package money

import "testing"

func TestMultiplierApply(t *testing.T) {
	bet := FromMajor(10)
	win := Multiplier(15000).Apply(bet) // 1.5x
	if win != FromMajor(15) {
		t.Fatalf("expected 15, got %s", win.String())
	}
}

func TestIntegerLedgerNoFloatDrift(t *testing.T) {
	bet := Amount(3333) // 0.3333
	mult := Multiplier(30000) // 3x
	win := mult.Apply(bet)
	if win != Amount(9999) {
		t.Fatalf("unexpected win: %d", win)
	}
}
