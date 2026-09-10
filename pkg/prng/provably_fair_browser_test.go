package prng

import (
	"testing"
)

// 固定向量 — web/verify/js/fair.js 必须使用相同输入输出
func TestProvablyFairKnownVector(t *testing.T) {
	roll, err := Roll("abc123seed", "client456", "round-test-1")
	if err != nil {
		t.Fatal(err)
	}
	if roll <= 0 || roll >= 1 {
		t.Fatalf("roll out of range: %f", roll)
	}

	hash := HashServerSeed("abc123seed")
	if len(hash) != 64 {
		t.Fatalf("expected 64 char hex hash, got %d", len(hash))
	}

	// 回归锚点：若 JS 验算页结果与此不一致，说明算法漂移
	const anchorRoll = 0.75434720333408778
	const tolerance = 1e-14
	if diff := roll - anchorRoll; diff > tolerance || diff < -tolerance {
		t.Fatalf("roll anchor mismatch: got %.17f want %.17f (sync web/verify/js/fair.js)", roll, anchorRoll)
	}

	const anchorHash = "d7e67bf1b02ad2c15e860ce392748c14279854cf69a74df10ccf11495f343c02"
	if hash != anchorHash {
		t.Fatalf("hash anchor mismatch: got %s", hash)
	}
}
