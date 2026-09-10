package logic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"fastgame/internal/model"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateBlacklistLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateBlacklistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBlacklistLogic {
	return &CreateBlacklistLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateBlacklistLogic) CreateBlacklist(req *types.CreateBlacklistReq) (*types.BlacklistItem, error) {
	switch req.ListType {
	case model.BlacklistTypeIP, model.BlacklistTypeUserID, model.BlacklistTypeMerchant:
	default:
		return nil, fmt.Errorf("invalid list type")
	}
	if req.ListValue == "" {
		return nil, fmt.Errorf("list value required")
	}

	var expiresAt sql.NullTime
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid expiresAt")
		}
		expiresAt = sql.NullTime{Time: t.UTC(), Valid: true}
	}

	row := &model.RiskBlacklist{
		ListType:  req.ListType,
		ListValue: req.ListValue,
		Reason:    req.Reason,
		Status:    1,
		ExpiresAt: expiresAt,
	}
	id, err := l.svcCtx.RiskBlacklist.Upsert(l.ctx, row)
	if err != nil {
		return nil, err
	}
	row.Id = id
	row.CreatedAt = time.Now().UTC()

	if err := l.svcCtx.Blacklist.SyncItem(l.ctx, row); err != nil {
		return nil, err
	}

	l.Infof("blacklist upserted: type=%s value=%s", req.ListType, req.ListValue)
	return toBlacklistItem(row), nil
}

func toBlacklistItem(row *model.RiskBlacklist) *types.BlacklistItem {
	item := &types.BlacklistItem{
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
	return item
}
