package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	Security SecurityConf
	Upload   UploadConf
	MySQL    MySQLConf
	Redis    RedisConf
	CH       ClickHouseConf
}

type UploadConf struct {
	Dir        string
	PublicBase string
	MaxBytes   int64
}

type SecurityConf struct {
	MerchantKeyCipher string
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
