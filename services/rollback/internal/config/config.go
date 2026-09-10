package config

import "time"

type Config struct {
	Kafka      KafkaConf
	ClickHouse ClickHouseConf
	MySQL      MySQLConf
	Wallet     WalletConf
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

type WalletConf struct {
	Mock                 bool
	BaseURL              string
	APIKey               string
	SignSecret           string
	SignEnabled          bool
	VerifyResponse       bool
	ResponseTimestampSec int
	Timeout              string
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
