package log

import (
	"os"

	"github.com/zeromicro/go-zero/core/logx"
)

// MustSetup initializes go-zero logging for any binary (REST or worker).
// Sets ServiceName, defaults Encoding to plain in dev when unset.
func MustSetup(serviceName string, c logx.LogConf) {
	if c.ServiceName == "" {
		c.ServiceName = serviceName
	}
	if c.Encoding == "" {
		if os.Getenv("FG_ENV") == "prod" || os.Getenv("GO_ENV") == "production" {
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
	logx.MustSetup(c)
	logx.AddGlobalFields(logx.Field(KeyService, serviceName))
}
