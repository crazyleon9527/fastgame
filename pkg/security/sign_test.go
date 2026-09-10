package security

import "testing"

func TestSignAndVerify(t *testing.T) {
	secret := "dev-secret-m001"
	payload := BuildSignPayload("POST", "/api/v1/game/bet", `{"betAmount":10}`, "1700000000", "nonce-1")
	sig := Sign(secret, payload)

	if err := VerifySign(secret, payload, sig); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if err := VerifySign(secret, payload, "bad-signature"); err == nil {
		t.Fatal("expected invalid signature error")
	}
}
