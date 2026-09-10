package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTraceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTraceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTraceLogic {
	return &GetTraceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTraceLogic) GetTrace(req *types.TraceLookupReq) (*types.TraceLookupResp, error) {
	spans, err := l.svcCtx.Reporter.QueryTraceSpans(l.ctx, req.TraceId)
	if err != nil {
		return nil, err
	}

	txs, err := l.svcCtx.PendingTx.FindByTraceID(l.ctx, req.TraceId)
	if err != nil {
		return nil, err
	}

	resp := &types.TraceLookupResp{TraceId: req.TraceId}
	for _, s := range spans {
		resp.Spans = append(resp.Spans, types.TraceSpanItem{
			SpanId:     s.SpanID,
			Service:    s.Service,
			Operation:  s.Operation,
			RoundId:    s.RoundID,
			Status:     s.Status,
			Detail:     s.Detail,
			DurationMs: s.DurationMs,
			OccurredAt: s.OccurredAt.Unix(),
		})
	}
	for _, tx := range txs {
		item := types.PendingTxItem{
			RoundId:        tx.RoundID,
			Phase:          tx.Phase,
			Status:         tx.Status,
			BetAmount:      tx.BetAmount,
			WinAmount:      tx.WinAmount,
			ExpectedAction: tx.ExpectedAction,
			RetryCount:     tx.RetryCount,
			CreatedAt:      tx.CreatedAt.Unix(),
		}
		if tx.LastError.Valid {
			item.LastError = tx.LastError.String
		}
		resp.PendingTransactions = append(resp.PendingTransactions, item)
	}
	return resp, nil
}
