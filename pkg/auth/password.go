package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordEmpty   = errors.New("password cannot be empty")
	ErrPasswordTooLong = errors.New("password exceeds maximum allowed length of 72 bytes")
)

// HashPassword 生成带安全成本的 bcrypt 哈希
func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", ErrPasswordEmpty
	}
	// bcrypt 会静默截断 72 字节之后的字符，因此显式拦截超长输入
	if len(password) > 72 {
		return "", ErrPasswordTooLong
	}
	raw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// CheckPassword 校验密码与哈希
func CheckPassword(hash, password string) bool {
	if len(password) == 0 || len(password) > 72 || len(hash) == 0 {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
