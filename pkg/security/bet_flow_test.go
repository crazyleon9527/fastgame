package security

import (
	"strconv"
	"testing"
	"time"

	"fastgame/pkg/session"
)

// TestSessionToBetEnvelopeFlow 验证 session → envelope 签名链（商户签名为 Skip，Envelope 强制）
func TestSessionToBetEnvelopeFlow(t *testing.T) {
	dynamicKey := "dynamic-session-key-test"
	roundID := "round-e2e-001"
	action := "cast"
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	sig := SignEnvelope(dynamicKey, roundID, action, ts)

	guard := NewGuard(Config{
		SkipMerchantSign:    true,
		SkipSessionEnvelope: false,
	}, nil, nil)

	if err := guard.VerifySessionEnvelope(dynamicKey, roundID, action, ts, sig); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
	if err := guard.VerifySessionEnvelope(dynamicKey, roundID, action, ts, "bad-signature"); err == nil {
		t.Fatal("expected invalid envelope to fail")
	}
	if err := guard.VerifySessionEnvelope(dynamicKey, roundID, "spin", ts, sig); err == nil {
		t.Fatal("expected action mismatch to fail")
	}
}

func TestSkipFlagsIndependent(t *testing.T) {
	guard := NewGuard(Config{
		SkipMerchantSign:    true,
		SkipSessionEnvelope: false,
	}, nil, nil)
	if guard.cfg.skipMerchant() != true || guard.cfg.skipEnvelope() != false {
		t.Fatal("skip flags should be independent")
	}

	guardLegacy := NewGuard(Config{SkipSignVerify: true}, nil, nil)
	if !guardLegacy.cfg.skipMerchant() || !guardLegacy.cfg.skipEnvelope() {
		t.Fatal("legacy SkipSignVerify should skip both")
	}
}

func TestEnvelopeMatchesSessionDynamicKey(t *testing.T) {
	// 模拟 CreateSession 后客户端用 DynamicSessionKey 签 envelope
	data := &session.Data{
		DynamicSessionKey: "sess-key-from-redis",
	}
	roundID := "round-abc"
	ts := "1700000000"
	sig := SignEnvelope(data.DynamicSessionKey, roundID, "cast", ts)
	if err := VerifyEnvelope(data.DynamicSessionKey, roundID, "cast", ts, sig); err != nil {
		t.Fatal(err)
	}
}
