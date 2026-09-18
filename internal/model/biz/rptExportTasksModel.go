package biz

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RptExportTasksModel = (*customRptExportTasksModel)(nil)

type (
	// RptExportTasksModel is an interface to be customized, add more methods here,
	// and implement the added methods in customRptExportTasksModel.
	RptExportTasksModel interface {
		rptExportTasksModel
	}

	customRptExportTasksModel struct {
		*defaultRptExportTasksModel
	}
)

// NewRptExportTasksModel returns a model for the database table.
func NewRptExportTasksModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RptExportTasksModel {
	return &customRptExportTasksModel{
		defaultRptExportTasksModel: newRptExportTasksModel(conn, c, opts...),
	}
}
