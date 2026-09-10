package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

const HeaderSessionSign = "X-Session-Sign"

// BuildEnvelopePayload Sign = HMAC-SHA256(RoundID + Action + Timestamp, DynamicSessionKey)
func BuildEnvelopePayload(roundID, action, timestamp string) string {
	return roundID + action + timestamp
}

func SignEnvelope(dynamicSessionKey, roundID, action, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(dynamicSessionKey))
	_, _ = mac.Write([]byte(BuildEnvelopePayload(roundID, action, timestamp)))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyEnvelope(dynamicSessionKey, roundID, action, timestamp, signature string) error {
	if dynamicSessionKey == "" || signature == "" {
		return fmt.Errorf("missing session envelope signature")
	}
	if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
		return fmt.Errorf("invalid envelope timestamp")
	}
	expected := SignEnvelope(dynamicSessionKey, roundID, action, timestamp)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("invalid session envelope signature")
	}
	return nil
}
