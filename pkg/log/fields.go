package log

// Standard structured log field keys (JSON snake_case for Loki/ELK).
const (
	KeyService    = "service"
	KeyTraceID    = "trace_id"
	KeyRoundID    = "round_id"
	KeyMerchantID = "merchant_id"
	KeyUserID     = "user_id"
	KeyGameCode   = "game_code"
	KeyTopic      = "topic"
	KeyPartition  = "partition"
	KeyOffset     = "offset"
	KeyMethod     = "method"
	KeyPath       = "path"
	KeyStatus     = "status"
	KeyDurationMs = "duration_ms"
	KeyClientIP   = "client_ip"
	KeyEvent      = "event"
	KeyErr        = "err"
)
