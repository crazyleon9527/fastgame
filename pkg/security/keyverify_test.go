package security

import (
	"database/sql"
	"testing"
	"time"

	"fastgame/internal/model"
)

func TestVerifyMerchantSignGracePeriod(t *testing.T) {
	oldSecret := "old-secret"
	newSecret := "new-secret"
	payload := BuildSignPayload("POST", "/api/v1/game/bet", `{}`, "1700000000", "n1")

	secrets := &model.MerchantSecrets{
		PrivateKey:              sql.NullString{String: newSecret, Valid: true},
		PrivateKeyPrev:          sql.NullString{String: oldSecret, Valid: true},
		PrivateKeyPrevExpiresAt: sql.NullTime{Time: time.Now().UTC().Add(time.Hour), Valid: true},
	}

	oldSig := Sign(oldSecret, payload)
	if err := VerifyMerchantSign(secrets, payload, oldSig); err != nil {
		t.Fatalf("old key should pass during grace: %v", err)
	}

	newSig := Sign(newSecret, payload)
	if err := VerifyMerchantSign(secrets, payload, newSig); err != nil {
		t.Fatalf("new key should pass: %v", err)
	}
}
