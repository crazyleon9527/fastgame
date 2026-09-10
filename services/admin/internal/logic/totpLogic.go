package logic

import (
	"context"
	"errors"
	"fmt"

	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"time"

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

func (l *TotpLogic) Confirm(userID uint64, req *types.TotpConfirmReq) error {
	secret, err := l.svcCtx.Redis.Get(l.ctx, fmt.Sprintf("admin:totp:pending:%d", userID)).Result()
	if err != nil || secret == "" {
		return errors.New("totp setup expired, run setup again")
	}
	if !auth.VerifyTOTP(secret, req.TotpCode) {
		return errors.New("invalid totp code")
	}
	if err := l.svcCtx.AdminAuth.UpdateTotp(l.ctx, userID, secret, true); err != nil {
		return err
	}
	_ = l.svcCtx.Redis.Del(l.ctx, fmt.Sprintf("admin:totp:pending:%d", userID)).Err()
	return nil
}
