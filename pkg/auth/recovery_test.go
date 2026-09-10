package auth

import "testing"

func TestRecoveryCodesConsumeOnce(t *testing.T) {
	plain, hashes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != recoveryCodeCount {
		t.Fatalf("expected %d codes", recoveryCodeCount)
	}
	jsonHashes, err := MarshalRecoveryHashes(hashes)
	if err != nil {
		t.Fatal(err)
	}
	newJSON, ok, err := ConsumeRecoveryCode(plain[0], jsonHashes)
	if err != nil || !ok {
		t.Fatalf("consume failed ok=%v err=%v", ok, err)
	}
	_, ok2, _ := ConsumeRecoveryCode(plain[0], newJSON)
	if ok2 {
		t.Fatal("code should be one-time use")
	}
}
