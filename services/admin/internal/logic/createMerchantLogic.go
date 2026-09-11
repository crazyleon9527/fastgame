package logic

import (
	"context"
	"database/sql"

	"fastgame/internal/model"
	"fastgame/pkg/fieldcipher"
	"fastgame/pkg/security"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateMerchantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateMerchantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMerchantLogic {
	return &CreateMerchantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateMerchantLogic) CreateMerchant(req *types.CreateMerchantReq) (*types.CreateMerchantResp, error) {
	status := req.Status
	if status == 0 {
		status = 1
	}

	privateKey, err := security.GenerateSecret()
	if err != nil {
		return nil, err
	}

	encKey, err := fieldcipher.Encrypt(privateKey)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.Merchants.Insert(l.ctx, &model.Merchants{
		MerchantCode: req.MerchantCode,
		Name:         req.Name,
		PrivateKey:   sql.NullString{String: encKey, Valid: true},
		Status:       status,
	})
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	l.Infof("merchant created: id=%d code=%s", id, req.MerchantCode)

	return &types.CreateMerchantResp{
		Id:           uint64(id),
		MerchantCode: req.MerchantCode,
		Name:         req.Name,
		Status:       status,
		PrivateKey:   privateKey,
	}, nil
}
