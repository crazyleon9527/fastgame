package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TotpLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTotpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TotpLogic {
	return &TotpLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *TotpLogic) Setup(userID uint64) (*types.TotpSetupResp, error) {
	user, err := l.svcCtx.AdminAuth.FindByID(l.ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	setup, err := auth.NewTotpSetup("FastGame-Sponge", user.Username)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Redis.Set(l.ctx, fmt.Sprintf("admin:totp:pending:%d", userID), setup.Secret, 10*time.Minute).Err(); err != nil {
		return nil, err
	}
	return &types.TotpSetupResp{
		ProvisioningUri: setup.Provisioning,
		Secret:          setup.Secret,
	}, nil
}

func (l *TotpLogic) Confirm(userID uint64, req *types.TotpConfirmReq) (*types.TotpConfirmResp, error) {
	secret, err := l.svcCtx.Redis.Get(l.ctx, fmt.Sprintf("admin:totp:pending:%d", userID)).Result()
	if err != nil || secret == "" {
		return nil, errors.New("totp setup expired, run setup again")
	}
	if !auth.VerifyTOTP(secret, req.TotpCode) {
		return nil, errors.New("invalid totp code")
	}
	if err := l.svcCtx.AdminAuth.UpdateTotp(l.ctx, userID, secret, true); err != nil {
		return nil, err
	}

	plain, hashes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		return nil, err
	}
	hashesJSON, err := auth.MarshalRecoveryHashes(hashes)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.AdminAuth.UpdateRecoveryHashes(l.ctx, userID, hashesJSON); err != nil {
		return nil, err
	}
	_ = l.svcCtx.Redis.Del(l.ctx, fmt.Sprintf("admin:totp:pending:%d", userID)).Err()

	user, err := l.svcCtx.AdminAuth.FindByID(l.ctx, userID)
	if err != nil {
		return nil, err
	}
	token, expireAt, err := issueAdminToken(user, l.svcCtx.Config.Auth.AccessSecret, l.svcCtx.Config.Auth.AccessExpire)
	if err != nil {
		return nil, err
	}

	return &types.TotpConfirmResp{
		AccessToken:   token,
		ExpireAt:      expireAt,
		RecoveryCodes: plain,
	}, nil
}
