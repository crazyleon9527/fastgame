package logic

import (
	"context"

	"fastgame/services/admin/internal/svc"
	"fastgame/services/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTraceByRoundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTraceByRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTraceByRoundLogic {
	return &GetTraceByRoundLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetTraceByRoundLogic) GetTraceByRound(req *types.TraceByRoundReq) (*types.TraceLookupResp, error) {
	spans, err := l.svcCtx.Reporter.QueryTraceSpansByRoundID(l.ctx, req.RoundId)
	if err != nil {
		return nil, err
	}

	tx, _ := l.svcCtx.PendingTx.FindByRoundID(l.ctx, req.RoundId)
	traceID := req.RoundId
	if len(spans) > 0 && spans[0].TraceID != "" {
		traceID = spans[0].TraceID
	}

	resp := &types.TraceLookupResp{TraceId: traceID, RoundId: req.RoundId}
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
	if tx != nil {
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
		if tx.TraceID != "" {
			resp.TraceId = tx.TraceID
		}
		resp.PendingTransactions = append(resp.PendingTransactions, item)
	}
	return resp, nil
}
