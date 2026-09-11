package logic

import (
	"context"
	"errors"

	"fastgame/internal/model"
	"fastgame/pkg/auth"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAdminUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAdminUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAdminUserLogic {
	return &CreateAdminUserLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateAdminUserLogic) CreateAdminUser(req *types.CreateAdminUserReq) (*types.AdminUserItem, error) {
	if _, err := l.svcCtx.Roles.FindNameByID(l.ctx, req.RoleId); err != nil {
		return nil, errors.New("invalid role")
	}
	if _, err := l.svcCtx.AdminUsers.FindOneByUsername(l.ctx, req.Username); err == nil {
		return nil, errors.New("username already exists")
	} else if err != model.ErrNotFound {
		return nil, err
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	status := req.Status
	if status == 0 {
		status = 1
	}

	result, err := l.svcCtx.AdminUsers.Insert(l.ctx, &model.AdminUsers{
		Username:     req.Username,
		PasswordHash: hash,
		RoleId:       req.RoleId,
		Status:       status,
	})
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	roleName, _ := l.svcCtx.Roles.FindNameByID(l.ctx, req.RoleId)

	l.Infof("admin user created: id=%d username=%s role=%s", id, req.Username, roleName)

	return &types.AdminUserItem{
		Id:       uint64(id),
		Username: req.Username,
		RoleId:   req.RoleId,
		RoleName: roleName,
		Status:   status,
	}, nil
}
