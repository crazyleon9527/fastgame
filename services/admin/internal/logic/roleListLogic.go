package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleListLogic {
	return &RoleListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RoleListLogic) RoleList() (*types.RoleListResp, error) {
	rows, err := l.svcCtx.Roles.ListAll(l.ctx)
	if err != nil {
		return nil, err
	}
	items := make([]types.RoleItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, types.RoleItem{
			Id:          row.Id,
			Name:        row.Name,
			Description: row.Description,
		})
	}
	return &types.RoleListResp{List: items}, nil
}
