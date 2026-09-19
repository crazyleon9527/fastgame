package config

import "github.com/zeromicro/go-zero/core/logx"

type Config struct {
	Log        logx.LogConf
	Kafka      KafkaConf
	ClickHouse ClickHouseConf
	MySQL      MySQLConf
	Redis      RedisConf
	RtpWatch   RtpWatchConf
	Batch      BatchConf
}

type KafkaConf struct {
	Brokers []string
	GroupID string
	Topic   string
	// RequireEnvelope 为 true 时要求消息是 outbox 信封格式（含 eventId/payload），
	// 用于防止"有人往 topic 里塞裸 JSON"绕过幂等与追踪。默认 false 以兼容旧消息。
	RequireEnvelope bool `json:",default=false"`
}

type ClickHouseConf struct {
	Addr     string
	Database string
	User     string
	Password string
}

type MySQLConf struct {
	DataSource string
}

type RedisConf struct {
	Addr string
}

type RtpWatchConf struct {
	Enabled      bool
	UseRedis     bool
	GlobalMax    int
	PlayerMax    int
	ThresholdPPM int64
	// MinSamples 触发告警所需的最少局数（0 = 用 pkg/rtpwatchdog 的统计默认值 2000）。
	// 调小它会让正常波动被误判成 RTP 漂移：窗口小时单局 20x 就能把窗口 RTP
	// 推到 180% 阈值以上。
	MinSamples int
	// AlertCooldown 同一对象两次告警的最小间隔（如 "10m"，0 = 默认 10m）。
	AlertCooldown string
}

type BatchConf struct {
	MaxSize  int
	Interval string
}
