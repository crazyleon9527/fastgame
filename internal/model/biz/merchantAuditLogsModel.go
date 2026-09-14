package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantAuditLogsModel = (*customMerchantAuditLogsModel)(nil)

type (
	// MerchantAuditLogsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantAuditLogsModel.
	MerchantAuditLogsModel interface {
		merchantAuditLogsModel
	}

	customMerchantAuditLogsModel struct {
		*defaultMerchantAuditLogsModel
	}
)

// NewMerchantAuditLogsModel returns a model for the database table.
func NewMerchantAuditLogsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantAuditLogsModel {
	return &customMerchantAuditLogsModel{
		defaultMerchantAuditLogsModel: newMerchantAuditLogsModel(conn, c, opts...),
	}
}
