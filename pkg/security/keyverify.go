package security

import (
	"fmt"
	"time"

	"fastgame/internal/model"
)

func VerifyMerchantSign(secrets *model.MerchantSecrets, payload, signature string) error {
	if secrets == nil {
		return fmt.Errorf("merchant secret missing")
	}
	if !secrets.PrivateKey.Valid || secrets.PrivateKey.String == "" {
		return fmt.Errorf("merchant secret not configured")
	}

	if err := VerifySign(secrets.PrivateKey.String, payload, signature); err == nil {
		return nil
	}

	if secrets.PrivateKeyPrev.Valid && secrets.PrivateKeyPrev.String != "" {
		if secrets.PrivateKeyPrevExpiresAt.Valid && time.Now().UTC().Before(secrets.PrivateKeyPrevExpiresAt.Time) {
			if err := VerifySign(secrets.PrivateKeyPrev.String, payload, signature); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("invalid signature")
}
