package fieldcipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const prefix = "enc:v1:"

var key []byte

func Init(passphrase string) {
	if passphrase == "" {
		key = nil
		return
	}
	sum := sha256.Sum256([]byte(passphrase))
	key = sum[:]
}

func Enabled() bool {
	return len(key) == 32
}

func Encrypt(plain string) (string, error) {
	if plain == "" || !Enabled() || strings.HasPrefix(plain, prefix) {
		return plain, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(stored string) (string, error) {
	if stored == "" || !strings.HasPrefix(stored, prefix) {
		return stored, nil
	}
	if !Enabled() {
		return "", errors.New("encrypted field but MerchantKeyCipher not configured")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", fmt.Errorf("decode field cipher: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt field: %w", err)
	}
	return string(plain), nil
}
