package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUserListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserListLogic {
	return &AdminUserListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminUserListLogic) AdminUserList(req *types.AdminUserListReq) (*types.AdminUserListResp, error) {
	list, total, err := l.svcCtx.AdminUsers.ListPage(l.ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]types.AdminUserItem, 0, len(list))
	for _, row := range list {
		items = append(items, types.AdminUserItem{
			Id:       row.Id,
			Username: row.Username,
			RoleId:   row.RoleId,
			RoleName: row.RoleName,
			Status:   row.Status,
		})
	}
	return &types.AdminUserListResp{Total: total, List: items}, nil
}
