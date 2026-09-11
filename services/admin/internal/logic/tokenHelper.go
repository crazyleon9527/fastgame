package logic

import (
	"context"
	"time"

	"fastgame/internal/model"

	"github.com/golang-jwt/jwt/v4"
)

func resolveRoleName(ctx context.Context, roles model.RolesModel, user *model.AdminAuthRecord) (string, error) {
	name, err := roles.FindNameByID(ctx, user.RoleId)
	if err != nil {
		if err == model.ErrNotFound {
			return "admin", nil
		}
		return "", err
	}
	return name, nil
}

func issueAdminToken(user *model.AdminAuthRecord, roleName, secret string, expireSeconds int64) (string, int64, error) {
	now := time.Now().Unix()
	expireAt := now + expireSeconds
	totpPending := user.TotpEnabled != 1
	if roleName == "" {
		roleName = "admin"
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":         expireAt,
		"iat":         now,
		"userId":      user.Id,
		"roleId":      user.RoleId,
		"roleName":    roleName,
		"totpPending": totpPending,
	})
	tokenStr, err := token.SignedString([]byte(secret))
	return tokenStr, expireAt, err
}
