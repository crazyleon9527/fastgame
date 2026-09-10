package logic

import (
	"context"
	"time"

	"fastgame/services/rgs/internal/svc"
	"fastgame/services/rgs/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionLogic {
	return &SessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SessionLogic) CreateSession(req *types.SessionReq) (*types.SessionResp, error) {
	data, err := l.svcCtx.Session.Create(l.ctx, req.MerchantId, req.UserId, req.GameCode, req.ClientSeed)
	if err != nil {
		return nil, err
	}

	return &types.SessionResp{
		SessionToken:      data.Token,
		ServerSeedHash:    data.ServerSeedHash,
		ClientSeed:        data.ClientSeed,
		DynamicSessionKey: data.DynamicSessionKey,
		NextSequenceId:    data.NextSequence,
		ExpiresAt:         time.Now().UTC().Add(l.svcCtx.Config.Session.TTL()).Unix(),
	}, nil
}
