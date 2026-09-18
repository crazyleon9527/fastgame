package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MerchantWalletDlqModel = (*customMerchantWalletDlqModel)(nil)

type (
	// MerchantWalletDlqModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMerchantWalletDlqModel.
	MerchantWalletDlqModel interface {
		merchantWalletDlqModel
	}

	customMerchantWalletDlqModel struct {
		*defaultMerchantWalletDlqModel
	}
)

// NewMerchantWalletDlqModel returns a model for the database table.
func NewMerchantWalletDlqModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MerchantWalletDlqModel {
	return &customMerchantWalletDlqModel{
		defaultMerchantWalletDlqModel: newMerchantWalletDlqModel(conn, c, opts...),
	}
}
