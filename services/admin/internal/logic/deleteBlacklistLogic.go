package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteBlacklistLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteBlacklistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteBlacklistLogic {
	return &DeleteBlacklistLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteBlacklistLogic) DeleteBlacklist(req *types.DeleteBlacklistReq) error {
	row, err := l.svcCtx.RiskBlacklist.FindOne(l.ctx, req.Id)
	if err != nil {
		return err
	}

	if err := l.svcCtx.RiskBlacklist.UpdateStatus(l.ctx, req.Id, 0); err != nil {
		return err
	}
	if err := l.svcCtx.Blacklist.Unblock(l.ctx, row.ListType, row.ListValue); err != nil {
		return err
	}

	l.Infof("blacklist removed: id=%d type=%s value=%s", req.Id, row.ListType, row.ListValue)
	return nil
}
