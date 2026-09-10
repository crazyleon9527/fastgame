package logic

import (
	"context"
	"time"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BlacklistListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBlacklistListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BlacklistListLogic {
	return &BlacklistListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BlacklistListLogic) BlacklistList(req *types.BlacklistListReq) (*types.BlacklistListResp, error) {
	list, total, err := l.svcCtx.RiskBlacklist.ListPage(l.ctx, req.ListType, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	items := make([]types.BlacklistItem, 0, len(list))
	for _, row := range list {
		item := types.BlacklistItem{
			Id:        row.Id,
			ListType:  row.ListType,
			ListValue: row.ListValue,
			Reason:    row.Reason,
			Status:    row.Status,
			CreatedAt: row.CreatedAt.UTC().Unix(),
		}
		if row.ExpiresAt.Valid {
			item.ExpiresAt = row.ExpiresAt.Time.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}

	return &types.BlacklistListResp{
		Total: total,
		List:  items,
	}, nil
}
