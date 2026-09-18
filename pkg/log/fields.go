package log

// Standard structured log field keys (JSON snake_case for Loki/ELK).
const (
	KeyService      = "service"
	KeyEnv          = "env"
	KeyTraceID      = "trace_id"
	KeySpanID       = "span_id"
	KeyRoundID      = "round_id"
	KeyMerchantID   = "merchant_id"
	KeyMerchantCode = "merchant_code"
	KeyUserID       = "user_id"
	KeyGameCode     = "game_code"
	KeyTopic        = "topic"
	KeyPartition    = "partition"
	KeyOffset       = "offset"
	KeyMethod       = "method"
	KeyPath         = "path"
	KeyStatus       = "status"
	KeyDurationMs   = "duration_ms"
	KeyDurationUs   = "duration_us"
	KeyClientIP     = "client_ip"
	KeyEvent        = "event"
	KeyErr          = "err"
)
