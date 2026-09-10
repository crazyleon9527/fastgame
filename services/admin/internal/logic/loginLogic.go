package logic

import (
	"context"
	"errors"

	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

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
		if req.RecoveryCode != "" {
			if !user.TotpRecoveryHashes.Valid {
				return nil, errors.New("no recovery codes configured")
			}
			newJSON, ok, err := auth.ConsumeRecoveryCode(req.RecoveryCode, user.TotpRecoveryHashes.String)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, errors.New("invalid recovery code")
			}
			if err := l.svcCtx.AdminAuth.ConsumeRecoveryHash(l.ctx, user.Id, newJSON); err != nil {
				return nil, err
			}
		} else {
			if req.TotpCode == "" {
				return nil, errors.New("totp code required")
			}
			if !user.TotpSecret.Valid || !auth.VerifyTOTP(user.TotpSecret.String, req.TotpCode) {
				return nil, errors.New("invalid totp code")
			}
		}
	}

	tokenStr, expireAt, err := issueAdminToken(user, l.svcCtx.Config.Auth.AccessSecret, l.svcCtx.Config.Auth.AccessExpire)
	if err != nil {
		return nil, err
	}

	return &types.LoginResp{
		AccessToken:       tokenStr,
		ExpireAt:          expireAt,
		RequiresTotpSetup: user.TotpEnabled != 1,
	}, nil
}
