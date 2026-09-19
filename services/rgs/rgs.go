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

	"fastgame/pkg/async"
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
	//
	// 这些循环以及退出流程都交给 async.Runner 管理：
	// 裸 goroutine 里的 panic 会直接打挂资金服务，进程退出时也无人等待收尾。
	bgCtx, stopBg := context.WithCancel(context.Background())
	defer stopBg()

	tasks := async.New("rgs-api")
	tasks.Run(bgCtx, func(runCtx context.Context) { ctx.OutboxDispatcher.Run(runCtx, time.Second) })
	tasks.Run(bgCtx, func(runCtx context.Context) {
		outbox.NewMaintainer(ctx.DB, outbox.MaintainerConfig{}).Run(runCtx, time.Minute)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	tasks.Run(context.Background(), func(context.Context) {
		<-sigCh
		logx.Info("[OUTBOX] shutdown signal received, stopping background loops")
		stopBg()
		server.Stop()
	})

	logx.Info("[OUTBOX] dispatcher and maintainer started (dispatch 1s, maintain 1m)")

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()

	// server.Start() 返回即进程要退出：等在途的后台任务收尾（含埋点写入）。
	// 超时只打日志，不静默丢弃——"有多少任务没跑完"是排障的关键信息。
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer drainCancel()
	if err := tasks.Shutdown(drainCtx); err != nil {
		logx.Errorf("[OUTBOX] shutdown drain: %v", err)
	}
	if err := ctx.Trace.Close(drainCtx); err != nil {
		logx.Errorf("[TRACE] shutdown drain: %v", err)
	}
}
