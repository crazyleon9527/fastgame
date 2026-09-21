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
	"fastgame/pkg/trace"
	"fastgame/pkg/xerr"
	"fastgame/services/rgs/internal/config"
	"fastgame/services/rgs/internal/handler"
	"fastgame/services/rgs/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/rgs-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 1. 注册全局标准化错误拦截器，自动绑定 xerr.CodeError 与 TraceID
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		traceID := trace.ID(ctx)
		ce := xerr.FromError(err)

		// 记录服务端内部错误链（脱敏面向前端的 msg，在日志中保留 cause）
		if ce.Cause() != nil {
			logx.WithContext(ctx).Errorf("[API-ERROR] code=%d msg=%s cause=%v", ce.Code(), ce.Msg(), ce.Cause())
		}

		resp := map[string]any{
			"code":    ce.Code(),
			"msg":     ce.Msg(),
			"traceId": traceID,
		}
		return ce.HttpStatus(), resp
	})

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	if c.Wallet.Mock || c.Security.SkipMerchantSign {
		logx.Errorf("[DEV-WARN] RGS running with insecure dev flags: Wallet.Mock=%v SkipMerchantSign=%v — do NOT use this config in production",
			c.Wallet.Mock, c.Security.SkipMerchantSign)
	}

	// 2. 启动后台协程池
	bgCtx, stopBg := context.WithCancel(context.Background())
	tasks := async.New("rgs-api")

	// 派发器：每 1 秒将 outbox 表中已落盘的事件投递到 Kafka
	tasks.Run(bgCtx, func(runCtx context.Context) {
		ctx.OutboxDispatcher.Run(runCtx, time.Second)
	})

	// 维护器：每 1 分钟通过 Redis 互斥锁单点回收挂起记录并清理历史数据
	tasks.Run(bgCtx, func(runCtx context.Context) {
		ctx.OutboxMaintainer.Run(runCtx, time.Minute)
	})

	logx.Info("[OUTBOX] dispatcher and maintainer started (dispatch 1s, maintain 1m)")

	// 3. 监听系统退出信号，执行受控的四阶段停机流程
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logx.Infof("[SHUTDOWN] signal %v received, initiating graceful draining...", sig)

		// 阶段 1：先切断入站流量，等待现有处理中的在途注单事务完成提交
		server.Stop()
		logx.Info("[SHUTDOWN] HTTP traffic drained")

		// 阶段 2：执行最后一轮强刷，把在途请求写入 outbox 表的事件送入 Kafka
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if n, err := ctx.OutboxDispatcher.DispatchOnce(flushCtx); err != nil {
			logx.Errorf("[SHUTDOWN] final outbox dispatch failed: %v", err)
		} else if n > 0 {
			logx.Infof("[SHUTDOWN] dispatched %d remaining outbox event(s)", n)
		}
		flushCancel()

		// 阶段 3：停止后台轮询任务
		stopBg()
	}()

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()

	// 阶段 4：server.Start() 返回后，回收在途协程与 ClickHouse 批量缓冲
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer drainCancel()

	if err := tasks.Shutdown(drainCtx); err != nil {
		logx.Errorf("[SHUTDOWN] async tasks drain error: %v", err)
	}

	if ctx.Trace != nil {
		if err := ctx.Trace.Close(drainCtx); err != nil {
			logx.Errorf("[SHUTDOWN] trace recorder flush error: %v", err)
		} else {
			logx.Info("[SHUTDOWN] trace recorder flushed cleanly")
		}
	}

	logx.Info("[SHUTDOWN] RGS server shutdown complete")
}
