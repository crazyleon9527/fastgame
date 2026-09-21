package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	recoveryCodeCount = 8
	recoveryCodeBytes = 5 // 5字节 = 10位16进制字符
)

// GenerateRecoveryCodes 生成一次性恢复码 (明文供用户保存，哈希存库)
func GenerateRecoveryCodes() (plain []string, hashes []string, err error) {
	plain = make([]string, 0, recoveryCodeCount)
	hashes = make([]string, 0, recoveryCodeCount)

	for i := 0; i < recoveryCodeCount; i++ {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		// 存储 SHA-256 哈希，避免每局/校验时重复调用昂贵的 bcrypt 导致 CPU 耗尽
		hash := hashRecoveryCode(code)
		plain = append(plain, formatCodeForDisplay(code))
		hashes = append(hashes, hash)
	}
	return plain, hashes, nil
}

func randomRecoveryCode() (string, error) {
	buf := make([]byte, recoveryCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

// 格式化为 XXXXX-XXXXX 便于用户辨认
func formatCodeForDisplay(code string) string {
	if len(code) == 10 {
		return code[:5] + "-" + code[5:]
	}
	return code
}

// 统一去除非字母数字符号并大写
func normalizeCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func hashRecoveryCode(code string) string {
	clean := normalizeCode(code)
	h := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(h[:])
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

	inputHash := hashRecoveryCode(code)
	targetIdx := -1

	// 恒定时间遍历比对，防止时序侧信道攻击
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(inputHash)) == 1 {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		return "", false, nil
	}

	// 移除已消费的恢复码
	remaining := append(hashes[:targetIdx], hashes[targetIdx+1:]...)
	newJSON, err := MarshalRecoveryHashes(remaining)
	if err != nil {
		return "", false, err
	}

	return newJSON, true, nil
}
