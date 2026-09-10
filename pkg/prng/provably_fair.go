package prng

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// Roll computes a deterministic [0,1) float from seeds using HMAC-SHA256.
func Roll(serverSeed, clientSeed, nonce string) (float64, error) {
	return RollIndex(serverSeed, clientSeed, nonce, 0)
}

func RollIndex(serverSeed, clientSeed, nonce string, index int) (float64, error) {
	if serverSeed == "" || clientSeed == "" || nonce == "" {
		return 0, fmt.Errorf("missing provably fair inputs")
	}

	payload := fmt.Sprintf("%s:%s:%d", clientSeed, nonce, index)
	mac := hmac.New(sha256.New, []byte(serverSeed))
	_, _ = mac.Write([]byte(payload))
	sum := mac.Sum(nil)

	n := binary.BigEndian.Uint64(sum[:8])
	return float64(n) / float64(math.MaxUint64), nil
}

func HashServerSeed(serverSeed string) string {
	sum := sha256.Sum256([]byte(serverSeed))
	return hex.EncodeToString(sum[:])
}
