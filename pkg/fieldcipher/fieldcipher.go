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
	"sync"
	"sync/atomic"
)

const (
	CurrentVersion = "v1"
	prefixV1       = "enc:v1:"
)

var (
	ErrNotConfigured   = errors.New("field cipher not configured")
	ErrCiphertextShort = errors.New("ciphertext too short")
	ErrUnknownVersion  = errors.New("unknown ciphertext version")
	ErrDecryptFailed   = errors.New("decrypt field failed: authentication error")
)

// Cipher 线程安全的字段加解密器
type Cipher struct {
	mu      sync.RWMutex
	primary cipher.AEAD
	history map[string]cipher.AEAD // 支持多版本密钥轮换: version -> AEAD
}

var globalCipher atomic.Pointer[Cipher]

// New 创建一个加解密器实例，主密钥由 passphrase 经 SHA-256 派生
func New(passphrase string, oldPassphrases ...string) (*Cipher, error) {
	if passphrase == "" {
		return nil, ErrNotConfigured
	}

	primaryAEAD, err := createAEAD(passphrase)
	if err != nil {
		return nil, err
	}

	c := &Cipher{
		primary: primaryAEAD,
		history: make(map[string]cipher.AEAD),
	}
	c.history[CurrentVersion] = primaryAEAD

	// 加载历史版本密钥，支持平滑轮换
	for i, old := range oldPassphrases {
		if old == "" {
			continue
		}
		oldAEAD, err := createAEAD(old)
		if err == nil {
			vKey := fmt.Sprintf("v%d_legacy", i+1)
			c.history[vKey] = oldAEAD
		}
	}

	return c, nil
}

func createAEAD(passphrase string) (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Init 全局单例初始化
func Init(passphrase string, oldPassphrases ...string) {
	if passphrase == "" {
		globalCipher.Store(nil)
		return
	}
	c, err := New(passphrase, oldPassphrases...)
	if err == nil {
		globalCipher.Store(c)
	}
}

// Enabled 检查当前加解密模块是否已激活
func Enabled() bool {
	c := globalCipher.Load()
	return c != nil && c.primary != nil
}

// Encrypt 加密字段并添加版本前缀
func Encrypt(plain string) (string, error) {
	c := globalCipher.Load()
	if c == nil {
		if !Enabled() {
			return plain, nil
		}
		return "", ErrNotConfigured
	}
	return c.Encrypt(plain)
}

// Decrypt 解密带有版本前缀的密文
func Decrypt(stored string) (string, error) {
	c := globalCipher.Load()
	if c == nil {
		if strings.HasPrefix(stored, "enc:") {
			return "", ErrNotConfigured
		}
		return stored, nil
	}
	return c.Decrypt(stored)
}

// Encrypt 实例加密
func (c *Cipher) Encrypt(plain string) (string, error) {
	// 避免对空值或重复加密字段操作
	if plain == "" || strings.HasPrefix(plain, "enc:") {
		return plain, nil
	}

	c.mu.RLock()
	aead := c.primary
	c.mu.RUnlock()

	if aead == nil {
		return plain, nil
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	// 密文结构: [nonce][ciphertext+tag]
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	encoded := base64.RawStdEncoding.EncodeToString(sealed)

	return prefixV1 + encoded, nil
}

// Decrypt 实例解密
func (c *Cipher) Decrypt(stored string) (string, error) {
	// 如果非加密格式或空字符串，原样返回（兼容存量未加密明文）
	if stored == "" || !strings.HasPrefix(stored, "enc:") {
		return stored, nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.primary == nil {
		return "", ErrNotConfigured
	}

	// 解析版本号，格式为 enc:<version>:<payload>
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("%w: invalid format", ErrUnknownVersion)
	}
	version := parts[1]
	payload := parts[2]

	raw, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	// 优先根据版本匹配解密密钥
	aead, ok := c.history[version]
	if !ok {
		aead = c.primary
	}

	if len(raw) < aead.NonceSize() {
		return "", ErrCiphertextShort
	}

	nonceSize := aead.NonceSize()
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]

	// 尝试解密
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err == nil {
		return string(plain), nil
	}

	// 如果指定版本解密失败，尝试通过历史旧密钥逐个重试解密（应对密钥平滑轮换）
	for v, altAEAD := range c.history {
		if v == version {
			continue
		}
		if p, altErr := altAEAD.Open(nil, nonce, ciphertext, nil); altErr == nil {
			return string(p), nil
		}
	}

	return "", fmt.Errorf("%w: %v", ErrDecryptFailed, err)
}
