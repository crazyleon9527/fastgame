package config

type Config struct {
	Kafka      KafkaConf
	ClickHouse ClickHouseConf
	MySQL      MySQLConf
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

type BatchConf struct {
	MaxSize  int
	Interval string
}
