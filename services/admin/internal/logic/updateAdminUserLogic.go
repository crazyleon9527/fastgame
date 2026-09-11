package logic

import (
	"context"
	"errors"

	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAdminUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAdminUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdminUserLogic {
	return &UpdateAdminUserLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateAdminUserLogic) UpdateAdminUser(req *types.UpdateAdminUserReq) (*types.AdminUserItem, error) {
	user, err := l.svcCtx.AdminUsers.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, errors.New("user not found")
	}

	currentID, _ := userIDFromCtx(l.ctx)
	if currentID == req.Id && req.Status == 2 {
		return nil, errors.New("cannot disable your own account")
	}

	if req.RoleId > 0 {
		if _, err := l.svcCtx.Roles.FindNameByID(l.ctx, req.RoleId); err != nil {
			return nil, errors.New("invalid role")
		}
		user.RoleId = req.RoleId
	}
	if req.Status == 1 || req.Status == 2 {
		user.Status = req.Status
	}
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = hash
	}

	if err := l.svcCtx.AdminUsers.Update(l.ctx, user); err != nil {
		return nil, err
	}

	roleName, _ := l.svcCtx.Roles.FindNameByID(l.ctx, user.RoleId)
	return &types.AdminUserItem{
		Id:       user.Id,
		Username: user.Username,
		RoleId:   user.RoleId,
		RoleName: roleName,
		Status:   user.Status,
	}, nil
}
