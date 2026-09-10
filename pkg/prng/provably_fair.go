package prng

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"sync"
)

var payloadPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 128)
		return &b
	},
}

// RollUint64 确定性 [0, MaxUint64] 整数 roll，热点路径零 float
func RollUint64(serverSeed, clientSeed, nonce string) (uint64, error) {
	return RollIndexUint64(serverSeed, clientSeed, nonce, 0)
}

func RollIndexUint64(serverSeed, clientSeed, nonce string, index int) (uint64, error) {
	if serverSeed == "" || clientSeed == "" || nonce == "" {
		return 0, fmt.Errorf("missing provably fair inputs")
	}

	bufPtr := payloadPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	buf = append(buf, clientSeed...)
	buf = append(buf, ':')
	buf = append(buf, nonce...)
	buf = append(buf, ':')
	buf = strconv.AppendInt(buf, int64(index), 10)

	mac := hmac.New(sha256.New, []byte(serverSeed))
	_, _ = mac.Write(buf)
	*bufPtr = buf
	payloadPool.Put(bufPtr)

	sum := mac.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8]), nil
}

// Roll 对外验算兼容 [0,1) float（仅 API 边界）
func Roll(serverSeed, clientSeed, nonce string) (float64, error) {
	n, err := RollUint64(serverSeed, clientSeed, nonce)
	if err != nil {
		return 0, err
	}
	return rollToFloat(n), nil
}

func RollIndex(serverSeed, clientSeed, nonce string, index int) (float64, error) {
	n, err := RollIndexUint64(serverSeed, clientSeed, nonce, index)
	if err != nil {
		return 0, err
	}
	return rollToFloat(n), nil
}

func rollToFloat(n uint64) float64 {
	return float64(n) / float64(math.MaxUint64)
}

// RollFloat 仅 API/验算边界将 uint64 roll 转为 [0,1) float
func RollFloat(n uint64) float64 {
	return rollToFloat(n)
}

func rollBelow(n uint64, num, den uint64) bool {
	if den == 0 {
		return false
	}
	return n < (math.MaxUint64/den)*num
}

func HashServerSeed(serverSeed string) string {
	sum := sha256.Sum256([]byte(serverSeed))
	return hex.EncodeToString(sum[:])
}
