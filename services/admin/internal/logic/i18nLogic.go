package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type I18nLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewI18nLogic(ctx context.Context, svcCtx *svc.ServiceContext) *I18nLogic {
	return &I18nLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *I18nLogic) LocaleList() (*types.LocaleListResp, error) {
	rows, err := l.svcCtx.I18n.ListLocales(l.ctx)
	if err != nil {
		return nil, err
	}
	items := make([]types.LocaleItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, types.LocaleItem{
			Code:       row.Code,
			Name:       row.Name,
			NativeName: row.NativeName,
			IsDefault:  row.IsDefault == 1,
		})
	}
	return &types.LocaleListResp{List: items}, nil
}

func (l *I18nLogic) Dictionary(req *types.I18nDictionaryReq) (*types.I18nDictionaryResp, error) {
	locale := req.Locale
	if locale == "" {
		locale = "en-US"
	}
	rows, err := l.svcCtx.I18n.ListDictionary(l.ctx, req.Bundle, locale)
	if err != nil {
		return nil, err
	}
	messages := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.TranslatedValue != "" {
			messages[row.MessageKey] = row.TranslatedValue
		} else {
			messages[row.MessageKey] = row.DefaultValue
		}
	}
	return &types.I18nDictionaryResp{
		Bundle:   req.Bundle,
		Locale:   locale,
		Messages: messages,
	}, nil
}
