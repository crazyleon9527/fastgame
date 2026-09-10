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
	CH       CHConf
	Wallet   WalletConf
	Game     GameConf
	Security SecurityConf
	Session  SessionConf
}

type CHConf struct {
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

type KafkaConf struct {
	Brokers []string
}

type WalletConf struct {
	Mock                 bool
	BaseURL              string
	APIKey               string
	SignSecret           string
	SignEnabled          bool
	VerifyResponse       bool
	ResponseTimestampSec int
	Timeout              string
	SlowThresholdMs      int
}

func (c WalletConf) SlowThreshold() time.Duration {
	if c.SlowThresholdMs <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(c.SlowThresholdMs) * time.Millisecond
}

func (c WalletConf) ResponseTimestampWindow() time.Duration {
	if c.ResponseTimestampSec <= 0 {
		return 60 * time.Second
	}
	if c.ResponseTimestampSec < 30 {
		return 30 * time.Second
	}
	if c.ResponseTimestampSec > 60 {
		return 60 * time.Second
	}
	return time.Duration(c.ResponseTimestampSec) * time.Second
}

type GameConf struct {
	DefaultRtpTier string
}

type SecurityConf struct {
	SkipSignVerify      bool `json:",optional"`
	SkipMerchantSign    bool `json:",optional"`
	SkipSessionEnvelope bool `json:",optional"`
	TimestampWindowSec  int
	MaxClockSkewSec     int
	UserLimitPerSec     int
	IPLimitPerSec       int
	MinResponseDelayMs  int
}

func (c SecurityConf) MaxClockSkew() time.Duration {
	if c.MaxClockSkewSec <= 0 {
		return 5 * time.Second
	}
	return time.Duration(c.MaxClockSkewSec) * time.Second
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
