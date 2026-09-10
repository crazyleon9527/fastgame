package prng

import (
	"testing"
)

func TestSecureFloatRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		v := secureFloat()
		if v < 0 || v >= 1 {
			t.Fatalf("secureFloat out of range: %f", v)
		}
	}
}

func TestSpinNeverNegativeWin(t *testing.T) {
	engine := NewEngine("high")
	for i := 0; i < 500; i++ {
		out := engine.Spin(10)
		if out.WinAmount < 0 || out.Multiplier < 0 {
			t.Fatalf("negative outcome: %+v", out)
		}
	}
}
