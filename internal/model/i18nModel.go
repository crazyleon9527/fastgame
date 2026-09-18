package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// LocaleRow 对应表 locales：平台支持语言
type LocaleRow struct {
	Code       string `db:"code"`        // BCP 47 语言标签，如 en-US、zh-CN
	Name       string `db:"name"`        // 英文显示名
	NativeName string `db:"native_name"` // 该语言自身的名称
	IsDefault  int64  `db:"is_default"`  // 平台默认回退语言
}

type I18nMessageRow struct {
	MessageKey      string `db:"message_key"`
	DefaultValue    string `db:"default_value"`
	TranslatedValue string `db:"translated_value"`
}

type I18nModel interface {
	ListLocales(ctx context.Context) ([]LocaleRow, error)
	ListDictionary(ctx context.Context, bundleCode, localeCode string) ([]I18nMessageRow, error)
}

type defaultI18nModel struct {
	conn sqlx.SqlConn
}

func NewI18nModel(conn sqlx.SqlConn) I18nModel {
	return &defaultI18nModel{conn: conn}
}

func (m *defaultI18nModel) ListLocales(ctx context.Context) ([]LocaleRow, error) {
	query := `select code, name, native_name, is_default from locales where status = 1 order by sort_order asc, code asc`
	var rows []LocaleRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *defaultI18nModel) ListDictionary(ctx context.Context, bundleCode, localeCode string) ([]I18nMessageRow, error) {
	query := fmt.Sprintf(`
select m.message_key, m.default_value, coalesce(t.translated_value, '') as translated_value
from i18n_messages m
join i18n_bundles b on b.id = m.bundle_id and b.bundle_code = ?
left join i18n_message_translations t on t.message_id = m.id and t.locale_code = ? and t.status = 1
where m.status = 1
order by m.message_key asc`)
	var rows []I18nMessageRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, bundleCode, localeCode); err != nil {
		return nil, err
	}
	return rows, nil
}
