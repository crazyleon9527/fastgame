package log

import (
	"context"

	"fastgame/pkg/trace"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

// InjectKafkaHeader injects trace_id from context into kafka message headers.
func InjectKafkaHeader(ctx context.Context, msg *kafka.Message) {
	if msg == nil {
		return
	}
	traceID := trace.ID(ctx)
	if traceID == "" {
		return
	}

	for i, h := range msg.Headers {
		if h.Key == trace.HeaderTraceID {
			msg.Headers[i].Value = []byte(traceID)
			return
		}
	}
	msg.Headers = append(msg.Headers, kafka.Header{
		Key:   trace.HeaderTraceID,
		Value: []byte(traceID),
	})
}

// ContextFromKafka builds trace + business correlation from message headers and payload fields.
func ContextFromKafka(ctx context.Context, msg kafka.Message, traceID, roundID, merchantCode, gameCode, userID string) context.Context {
	if tid := traceIDFromMessage(msg); tid != "" {
		traceID = tid
	}
	if traceID != "" {
		ctx = trace.WithID(ctx, traceID)
	}

	fields := make([]logx.LogField, 0, 7)
	if traceID != "" {
		fields = append(fields, logx.Field(KeyTraceID, traceID))
	}
	if roundID != "" {
		fields = append(fields, logx.Field(KeyRoundID, roundID))
	}
	if merchantCode != "" {
		fields = append(fields, logx.Field(KeyMerchantCode, merchantCode))
	}
	if gameCode != "" {
		fields = append(fields, logx.Field(KeyGameCode, gameCode))
	}
	if userID != "" {
		fields = append(fields, logx.Field(KeyUserID, userID))
	}
	if msg.Topic != "" {
		fields = append(fields, logx.Field(KeyTopic, msg.Topic))
	}
	if len(msg.Headers) > 0 {
		fields = append(fields,
			logx.Field(KeyPartition, msg.Partition),
			logx.Field(KeyOffset, msg.Offset),
		)
	}

	if len(fields) == 0 {
		return ctx
	}
	return WithFields(ctx, fields...)
}

func traceIDFromMessage(msg kafka.Message) string {
	for _, h := range msg.Headers {
		if h.Key == trace.HeaderTraceID {
			return string(h.Value)
		}
	}
	return ""
}
