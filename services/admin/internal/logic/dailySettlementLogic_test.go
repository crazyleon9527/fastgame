package logic

import "testing"

func TestCalcRtp(t *testing.T) {
	if got := calcRtp(0, 100); got != 0 {
		t.Fatalf("calcRtp(0,100)=%v want 0", got)
	}
	if got := calcRtp(10000, 9600); got != 0.96 {
		t.Fatalf("calcRtp(10000,9600)=%v want 0.96", got)
	}
}

func TestMinorToMajor(t *testing.T) {
	if got := minorToMajor(123450000); got != 12345 {
		t.Fatalf("minorToMajor(123450000)=%v want 12345", got)
	}
}
