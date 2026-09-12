package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuditLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuditLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditLogLogic {
	return &AuditLogLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AuditLogLogic) List(req *types.AuditLogListReq) (*types.AuditLogListResp, error) {
	list, total, err := l.svcCtx.AuditLogs.ListPage(l.ctx, req.Action, req.Username, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]types.AuditLogItem, 0, len(list))
	for _, row := range list {
		item := types.AuditLogItem{
			Id:           row.Id,
			Username:     row.Username,
			RoleName:     row.RoleName,
			Action:       row.Action,
			ResourceType: row.ResourceType,
			ResourceId:   row.ResourceId,
			CreatedAt:    row.CreatedAt.UnixMilli(),
		}
		if row.AdminUserId.Valid {
			item.AdminUserId = uint64(row.AdminUserId.Int64)
		}
		if row.HttpMethod.Valid {
			item.HttpMethod = row.HttpMethod.String
		}
		if row.RequestPath.Valid {
			item.RequestPath = row.RequestPath.String
		}
		if row.Detail.Valid {
			item.Detail = row.Detail.String
		}
		if row.ClientIp.Valid {
			item.ClientIp = row.ClientIp.String
		}
		if row.StatusCode.Valid {
			item.StatusCode = int(row.StatusCode.Int64)
		}
		items = append(items, item)
	}
	return &types.AuditLogListResp{Total: total, List: items}, nil
}
