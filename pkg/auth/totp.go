package auth

import (
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// VerifyTOTP 校验 TOTP 动态码，默认允许 ±1 个步长（30秒）的时钟漂移
func VerifyTOTP(secret, code string) bool {
	return VerifyTOTPCustom(secret, code, time.Now().UTC())
}

// VerifyTOTPCustom 支持指定时间比对，方便单元测试
func VerifyTOTPCustom(secret, code string, t time.Time) bool {
	if secret == "" || len(code) != 6 {
		return false
	}
	valid, err := totp.ValidateCustom(code, secret, t, totp.ValidateOpts{
		Period:    30,
		Skew:      1, // 允许前一个步长、当前步长、后一个步长 (共 90 秒容错)
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}
