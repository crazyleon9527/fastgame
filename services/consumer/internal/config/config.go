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
}

type BatchConf struct {
	MaxSize  int
	Interval string
}
