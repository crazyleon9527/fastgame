package config

import (
	"time"

	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	MySQL    MySQLConf
	Redis    RedisConf
	Kafka    KafkaConf
	Wallet   WalletConf
	Game     GameConf
	Security SecurityConf
	Session  SessionConf
}

type MySQLConf struct {
	DataSource string
}

type RedisConf struct {
	Addr string
}

type KafkaConf struct {
	Brokers []string
}

type WalletConf struct {
	Mock    bool
	BaseURL string
	APIKey  string
	Timeout string
}

type GameConf struct {
	DefaultRtpTier string
}

type SecurityConf struct {
	SkipSignVerify     bool
	TimestampWindowSec int
	UserBetLimitPerMin int
	IPLimitPerSec      int
	MinResponseDelayMs int
}

func (c SecurityConf) TimestampWindow() time.Duration {
	if c.TimestampWindowSec <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.TimestampWindowSec) * time.Second
}

func (c SecurityConf) MinResponseDelay() time.Duration {
	if c.MinResponseDelayMs <= 0 {
		return 0
	}
	return time.Duration(c.MinResponseDelayMs) * time.Millisecond
}

type SessionConf struct {
	TTLHours int
}

func (c SessionConf) TTL() time.Duration {
	if c.TTLHours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(c.TTLHours) * time.Hour
}
