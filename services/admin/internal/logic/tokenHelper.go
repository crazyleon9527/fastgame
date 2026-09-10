package logic

import (
	"time"

	"fastgame/internal/model"

	"github.com/golang-jwt/jwt/v4"
)

func issueAdminToken(user *model.AdminAuthRecord, secret string, expireSeconds int64) (string, int64, error) {
	now := time.Now().Unix()
	expireAt := now + expireSeconds
	totpPending := user.TotpEnabled != 1
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":         expireAt,
		"iat":         now,
		"userId":      user.Id,
		"roleId":      user.RoleId,
		"totpPending": totpPending,
	})
	tokenStr, err := token.SignedString([]byte(secret))
	return tokenStr, expireAt, err
}
