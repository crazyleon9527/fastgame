package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	MySQL MySQLConf
	Redis RedisConf
	CH    ClickHouseConf
}

type RedisConf struct {
	Addr string
}

type MySQLConf struct {
	DataSource string
}

type ClickHouseConf struct {
	Addr     string
	Database string
	User     string
	Password string
}
