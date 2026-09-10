package config

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
	Mock    bool
	BaseURL string
	APIKey  string
	Timeout string
}
