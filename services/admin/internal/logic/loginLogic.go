package logic

import (
	"context"
	"errors"
	"time"

	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	user, err := l.svcCtx.AdminAuth.FindByUsername(l.ctx, req.Username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}
	if user.Status != 1 || !auth.CheckPassword(user.PasswordHash, req.Password) {
		return nil, errors.New("invalid username or password")
	}

	if user.TotpEnabled == 1 {
		if req.TotpCode == "" {
			return nil, errors.New("totp code required")
		}
		if !user.TotpSecret.Valid || !auth.VerifyTOTP(user.TotpSecret.String, req.TotpCode) {
			return nil, errors.New("invalid totp code")
		}
	}

	now := time.Now().Unix()
	expireAt := now + l.svcCtx.Config.Auth.AccessExpire
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":    expireAt,
		"iat":    now,
		"userId": user.Id,
		"roleId": user.RoleId,
	})
	tokenStr, err := token.SignedString([]byte(l.svcCtx.Config.Auth.AccessSecret))
	if err != nil {
		return nil, err
	}

	return &types.LoginResp{
		AccessToken: tokenStr,
		ExpireAt:    expireAt,
	}, nil
}
