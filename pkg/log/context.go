package log

import (
	"context"

	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/logx"
)

// RoundFields encapsulates correlation identifiers for a game round.
type RoundFields struct {
	MerchantID   uint64
	MerchantCode string
	GameCode     string
	RoundId      string
	UserId       string
}

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

// WithRoundContext binds the primary gaming context to the request.
func WithRoundContext(ctx context.Context, rf RoundFields) context.Context {
	fields := make([]logx.LogField, 0, 5)
	if rf.RoundId != "" {
		fields = append(fields, logx.Field(KeyRoundID, rf.RoundId))
	}
	if rf.MerchantCode != "" {
		fields = append(fields, logx.Field(KeyMerchantCode, rf.MerchantCode))
	}
	if rf.MerchantID != 0 {
		fields = append(fields, logx.Field(KeyMerchantID, rf.MerchantID))
	}
	if rf.GameCode != "" {
		fields = append(fields, logx.Field(KeyGameCode, rf.GameCode))
	}
	if rf.UserId != "" {
		fields = append(fields, logx.Field(KeyUserID, rf.UserId))
	}
	if len(fields) == 0 {
		return ctx
	}
	return WithFields(ctx, fields...)
}

func WithRound(ctx context.Context, roundID string) context.Context {
	if roundID == "" {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyRoundID, roundID))
}

func WithMerchantCode(ctx context.Context, merchantCode string) context.Context {
	if merchantCode == "" {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyMerchantCode, merchantCode))
}

func WithMerchant(ctx context.Context, merchantID uint64) context.Context {
	if merchantID == 0 {
		return ctx
	}
	return WithFields(ctx, logx.Field(KeyMerchantID, merchantID))
}

// WithUser binds external user ID (string).
func WithUser(ctx context.Context, userID string) context.Context {
	if userID == "" {
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
