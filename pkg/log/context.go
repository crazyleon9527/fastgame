package log

import (
	"context"

	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/logx"
)

// C returns a logger with trace_id and any context-bound fields attached.
func C(ctx context.Context) logx.Logger {
	return logx.WithContext(EnsureTrace(ctx))
}

// EnsureTrace copies pkg/trace ID into logx context fields when missing.
func EnsureTrace(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if trace.ID(ctx) == "" {
		return ctx
	}
	return WithFields(ctx,
		logx.Field(KeyTraceID, trace.ID(ctx)),
	)
}

// WithFields merges structured fields into the logging context.
func WithFields(ctx context.Context, fields ...logx.LogField) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return logx.ContextWithFields(ctx, fields...)
}

func WithRound(ctx context.Context, roundID string) context.Context {
	if roundID == "" {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyRoundID, roundID))
}

func WithMerchant(ctx context.Context, merchantID string) context.Context {
	if merchantID == "" {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyMerchantID, merchantID))
}

func WithUser(ctx context.Context, userID uint64) context.Context {
	if userID == 0 {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyUserID, userID))
}

func WithGame(ctx context.Context, gameCode string) context.Context {
	if gameCode == "" {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyGameCode, gameCode))
}
