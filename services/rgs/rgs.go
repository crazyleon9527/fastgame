// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fastgame/pkg/outbox"
	"fastgame/services/rgs/internal/config"
	"fastgame/services/rgs/internal/handler"
	"fastgame/services/rgs/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/rgs-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	if c.Wallet.Mock || c.Security.SkipMerchantSign {
		logx.Errorf("[DEV-WARN] RGS running with insecure dev flags: Wallet.Mock=%v SkipMerchantSign=%v — do NOT use this config in production",
			c.Wallet.Mock, c.Security.SkipMerchantSign)
	}

	// outbox 后台循环：派发结算事件 + 回收悬挂记录/清理历史。
	// 用独立 ctx，随进程收到 SIGINT/SIGTERM 时优雅停止。
	bgCtx, stopBg := context.WithCancel(context.Background())
	defer stopBg()

	go ctx.OutboxDispatcher.Run(bgCtx, time.Second)
	go outbox.NewMaintainer(ctx.DB, outbox.MaintainerConfig{}).Run(bgCtx, time.Minute)
	logx.Info("[OUTBOX] dispatcher and maintainer started (dispatch 1s, maintain 1m)")

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logx.Info("[OUTBOX] shutdown signal received, stopping background loops")
		stopBg()
		server.Stop()
	}()

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
