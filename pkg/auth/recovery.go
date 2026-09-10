package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const recoveryCodeCount = 8

// GenerateRecoveryCodes 生成一次性恢复码及 bcrypt 哈希（仅存哈希）
func GenerateRecoveryCodes() (plain []string, hashes []string, err error) {
	plain = make([]string, 0, recoveryCodeCount)
	hashes = make([]string, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		hash, err := HashPassword(code)
		if err != nil {
			return nil, nil, err
		}
		plain = append(plain, code)
		hashes = append(hashes, hash)
	}
	return plain, hashes, nil
}

func randomRecoveryCode() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

func MarshalRecoveryHashes(hashes []string) (string, error) {
	raw, err := json.Marshal(hashes)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func ParseRecoveryHashes(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var hashes []string
	if err := json.Unmarshal([]byte(raw), &hashes); err != nil {
		return nil, err
	}
	return hashes, nil
}

// ConsumeRecoveryCode 校验并消耗一个恢复码，返回更新后的哈希 JSON
func ConsumeRecoveryCode(code string, hashesJSON string) (newHashesJSON string, ok bool, err error) {
	hashes, err := ParseRecoveryHashes(hashesJSON)
	if err != nil || len(hashes) == 0 {
		return "", false, fmt.Errorf("no recovery codes configured")
	}
	code = strings.TrimSpace(strings.ToUpper(code))
	for i, h := range hashes {
		if CheckPassword(h, code) { // hash, plain code
			remaining := append(hashes[:i], hashes[i+1:]...)
			newJSON, err := MarshalRecoveryHashes(remaining)
			return newJSON, true, err
		}
	}
	return "", false, nil
}
