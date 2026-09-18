package main

import (
	"fmt"
	"net/http"

	applog "fastgame/pkg/log"
	"fastgame/pkg/money"
	"fastgame/pkg/wallet"

	"github.com/zeromicro/go-zero/core/logx"
)

func main() {
	applog.MustSetup("mock-wallet", logx.LogConf{
		ServiceName: "mock-wallet",
		Mode:        "console",
		Level:       "info",
		Encoding:    "plain",
	})

	mockEngine := wallet.NewMockClient(money.FromMajor(10000))
	secret := "test_secret_key_8888"

	mux := http.NewServeMux()
	mockEngine.RegisterHTTPRoutes(mux)

	// 组装洋葱式中间件链:
	// HTTP 日志中间件 -> 故障注入中间件 -> 签名校验与防重放中间件 -> 业务路由
	var handler http.Handler = mux
	handler = wallet.VerifySignatureMiddleware(secret)(handler)
	handler = wallet.FaultInjectionMiddleware()(handler)
	handler = http.HandlerFunc(applog.HTTPMiddleware()(handler.ServeHTTP))

	port := 8089
	logx.Infof("🚀 工业级 Seamless Mock 钱包服务已启动, 监听端口: :%d", port)
	logx.Infof("👉 支持故障注入测试头: %s (可选值: 504_GATEWAY_TIMEOUT, 500_INTERNAL_ERROR, HANG_TIMEOUT)", wallet.HeaderMockFault)
	logx.Infof("👉 支持延迟注入测试头: %s (单位: 毫秒)", wallet.HeaderMockLatency)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		panic(err)
	}
}
