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
