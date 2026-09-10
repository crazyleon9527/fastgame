package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	MySQL  MySQLConf
	Redis  RedisConf
	Kafka  KafkaConf
	Wallet WalletConf
	Game   GameConf
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
