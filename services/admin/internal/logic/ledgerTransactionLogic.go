package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"fastgame/internal/model"
	"fastgame/pkg/ledger"
	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// LedgerErrStatus 把账变相关的哨兵错误映射成 HTTP 状态码 + 给后台看的中文提示。
//
// 为什么在 logic 里映射而不是直接上抛给 httpx：go-zero 默认把任何业务错误写成
// 400（httpx/requests.go 的 doHandleError），那样数据库故障会伪装成"参数错误"，
// 后台排查会走错方向。这里显式区分"调用方可修复"与"服务端故障"。
func LedgerErrStatus(err error) (int, string) {
	switch {
	case errors.Is(err, ledger.ErrAmountNotPositive):
		return http.StatusBadRequest, "金额必须为正数（方向由账变类型决定，调用方只传正数金额）"
	case errors.Is(err, ledger.ErrInsufficientBalance):
		return http.StatusBadRequest, "余额不足：本地镜像余额小于本次扣款额，说明本地台账与钱包状态不一致，必须人工核查后再调账"
	case errors.Is(err, model.ErrTransactionTypeNotFound):
		return http.StatusBadRequest, "账变类型不可用：该类型不存在或已停用"
	case errors.Is(err, ErrAdjustTypeUnsupported):
		return http.StatusBadRequest, ErrAdjustTypeUnsupported.Error()
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// 客户端断开或超时，账变可能已落库，提示调用方用 externalTxId 复核而不是盲目重试。
		return http.StatusInternalServerError, "请求已取消或超时，请用 externalTxId 核对该笔人工调账是否已落账"
	default:
		return http.StatusInternalServerError, err.Error()
	}
}

type LedgerTransactionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLedgerTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LedgerTransactionLogic {
	return &LedgerTransactionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// List 账变流水查询（后台只读）。
func (l *LedgerTransactionLogic) List(req *types.LedgerTransactionListReq) (*types.LedgerTransactionListResp, error) {
	from, err := parseLedgerTime(req.StartTime)
	if err != nil {
		return nil, fmt.Errorf("startTime 不正确: %w", err)
	}
	to, err := parseLedgerTime(req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("endTime 不正确: %w", err)
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return nil, errors.New("startTime 不能晚于 endTime")
	}

	// page/pageSize 的默认值与上限由 model.ListTransactions 兜底，
	// 这里只负责如实回显（回显用规范化后的值，客户端才能按同样的口径翻页）。
	page, pageSize := req.Page, req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	query := model.LedgerQuery{
		MerchantId: req.MerchantId,
		UserId:     req.UserId,
		TxType:     req.TxType,
		RoundId:    req.RoundId,
		Status:     req.Status,
		DriftOnly:  req.DriftOnly,
		Page:       page,
		PageSize:   pageSize,
	}
	if !from.IsZero() {
		query.From = &from
	}
	if !to.IsZero() {
		query.To = &to
	}

	list, total, err := l.svcCtx.LedgerModel.ListTransactions(l.ctx, l.svcCtx.DB, query)
	if err != nil {
		return nil, err
	}

	typeNames := l.typeNameMap()

	items := make([]types.LedgerTransactionItem, 0, len(list))
	for _, row := range list {
		item := types.LedgerTransactionItem{
			Id:            row.Id,
			TransactionId: row.TransactionId,
			ExternalTxId:  row.ExternalTxId,
			TxType:        row.TxType,
			Direction:     row.Direction,
			MerchantId:    row.MerchantId,
			MerchantCode:  row.MerchantCode,
			UserId:        row.UserId,
			GameCode:      row.GameCode,
			Currency:      row.Currency,
			Amount:        row.Amount,
			BalanceBefore: row.BalanceBefore,
			BalanceAfter:  row.BalanceAfter,
			DriftMinor:    row.DriftMinor,
			Status:        row.Status,
			Remark:        row.Remark,
			TypeName:      typeNames[row.TxType],
			CreatedAt:     row.CreatedAt.Unix(),
		}
		if row.RoundId.Valid {
			item.RoundId = row.RoundId.String
		}
		items = append(items, item)
	}

	return &types.LedgerTransactionListResp{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// typeNameMap 一次性把启用的账变类型查成 code→中文名 的映射。
//
// 用 ListEnabledTypes 而不是逐条 FindEnabledTypeByCode：一页最多 500 条流水，
// 逐条查就是 500 次往返（N+1）。类型表只有个位数行，一次查完即可。
func (l *LedgerTransactionLogic) typeNameMap() map[string]string {
	rows, err := l.svcCtx.LedgerModel.ListEnabledTypes(l.ctx, l.svcCtx.DB)
	if err != nil {
		// 类型名只是展示字段：查不到不该让整个流水列表失败，
		// 降级成空名字（流水本身仍然可读）并留日志。
		l.Errorf("查询账变类型失败，typeName 本次留空: %v", err)
		return map[string]string{}
	}
	names := make(map[string]string, len(rows))
	for _, t := range rows {
		names[t.Code] = t.Name
	}
	return names
}
