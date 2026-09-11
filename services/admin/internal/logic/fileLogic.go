package logic

import (
	"context"
	"mime/multipart"

	"fastgame/pkg/adminupload"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FileLogic {
	return &FileLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *FileLogic) UploadAsset(header *multipart.FileHeader) (*types.FileUploadResp, error) {
	saved, err := adminupload.SaveUploaded(
		l.svcCtx.UploadDir,
		l.svcCtx.UploadPublicBase,
		header,
		adminupload.AllowedImage,
	)
	if err != nil {
		return nil, err
	}
	return &types.FileUploadResp{
		URL:      saved.URL,
		Filename: saved.Filename,
		Size:     saved.Size,
	}, nil
}

func (l *FileLogic) UploadExcel(header *multipart.FileHeader) (*types.FileUploadResp, error) {
	saved, err := adminupload.SaveUploaded(
		l.svcCtx.UploadDir,
		l.svcCtx.UploadPublicBase,
		header,
		adminupload.AllowedExcel,
	)
	if err != nil {
		return nil, err
	}
	return &types.FileUploadResp{
		URL:      saved.URL,
		Filename: saved.Filename,
		Size:     saved.Size,
	}, nil
}
