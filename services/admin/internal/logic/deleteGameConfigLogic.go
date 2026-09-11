package logic

import (
	"context"
	"errors"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteGameConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteGameConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteGameConfigLogic {
	return &DeleteGameConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteGameConfigLogic) DeleteGameConfig(id uint64) error {
	if _, err := l.svcCtx.GameConfigs.FindOne(l.ctx, id); err != nil {
		if err == model.ErrNotFound {
			return errors.New("game config not found")
		}
		return err
	}
	return l.svcCtx.GameConfigs.Delete(l.ctx, id)
}
