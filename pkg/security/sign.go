package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// BuildSignPayload returns canonical string for HMAC signing.
// method|path|body|timestamp|nonce
func BuildSignPayload(method, path, body, timestamp, nonce string) string {
	return strings.Join([]string{
		strings.ToUpper(method),
		path,
		body,
		timestamp,
		nonce,
	}, "|")
}

func Sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifySign(secret, payload, signature string) error {
	if secret == "" {
		return fmt.Errorf("merchant secret missing")
	}
	expected := Sign(secret, payload)
	if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(signature))) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}
