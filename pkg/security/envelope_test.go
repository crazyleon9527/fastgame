package security

import "testing"

func TestVerifyEnvelope(t *testing.T) {
	key := "dynamic-session-key-abc"
	ts := "1700000000"
	sig := SignEnvelope(key, "round-1", "cast", ts)
	if err := VerifyEnvelope(key, "round-1", "cast", ts, sig); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEnvelope(key, "round-1", "cast", ts, "bad"); err == nil {
		t.Fatal("expected invalid signature")
	}
}
