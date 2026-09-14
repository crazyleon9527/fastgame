package log

import (
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// MustSetup initializes go-zero logging for any binary (REST or worker).
func MustSetup(serviceName string, c logx.LogConf) {
	if c.ServiceName == "" {
		c.ServiceName = serviceName
	}

	env := strings.ToLower(os.Getenv("FG_ENV"))
	if env == "" {
		env = strings.ToLower(os.Getenv("GO_ENV"))
	}
	if env == "" {
		env = "dev"
	}

	if c.Encoding == "" {
		if env == "prod" || env == "production" {
			c.Encoding = "json"
		} else {
			c.Encoding = "plain"
		}
	}
	if c.Mode == "" {
		c.Mode = "console"
	}
	if c.Level == "" {
		c.Level = "info"
	}

	// 允许通过环境变量 FG_LOG_LEVEL 动态覆盖配置级别
	if lvl := os.Getenv("FG_LOG_LEVEL"); lvl != "" {
		c.Level = strings.ToLower(lvl)
	}

	logx.MustSetup(c)
	logx.AddGlobalFields(
		logx.Field(KeyService, serviceName),
		logx.Field(KeyEnv, env),
	)
}
