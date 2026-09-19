package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// MustSetup 初始化 go-zero 日志
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

	// 1. 默认编码规则
	if c.Encoding == "" {
		if env == "prod" || env == "production" {
			c.Encoding = "json"
		} else {
			c.Encoding = "plain"
		}
	}

	// 2. 模式与多服务日志目录隔离
	if c.Mode == "" {
		c.Mode = "console"
	} else if c.Mode == "file" {
		// 默认保存在 logs/<serviceName>/ 下，避免多个微服务日志串入同一个文件
		if c.Path == "" {
			c.Path = filepath.Join("logs", serviceName)
		} else if !strings.HasSuffix(c.Path, serviceName) {
			c.Path = filepath.Join(c.Path, serviceName)
		}

		// 默认按天归档并保留 7 天
		if c.Rotation == "" {
			c.Rotation = "daily"
		}
		if c.KeepDays <= 0 {
			c.KeepDays = 7
		}
	}

	if c.Level == "" {
		c.Level = "info"
	}
	if lvl := os.Getenv("FG_LOG_LEVEL"); lvl != "" {
		c.Level = strings.ToLower(lvl)
	}

	logx.MustSetup(c)
	logx.AddGlobalFields(
		logx.Field(KeyService, serviceName),
		logx.Field(KeyEnv, env),
	)

	fmt.Printf("[LOG-INIT] Service=%s Env=%s Mode=%s Path=%s Level=%s Rotation=%s\n",
		c.ServiceName, env, c.Mode, c.Path, c.Level, c.Rotation)
}
