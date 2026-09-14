package wallet

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	applog "fastgame/pkg/log"
	"fastgame/pkg/security"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	HeaderMockFault   = "X-Mock-Fault"   // 模拟故障类型: 504, 500, timeout
	HeaderMockLatency = "X-Mock-Latency" // 强制注入网络延迟 (ms)
	MaxTimeSkewSec    = 60               // 允许的最大时钟偏差 (秒)
)

// VerifySignatureMiddleware 服务端入站签名防重放中间件
// 适用于 Mock 钱包服务端，以及未来 RGS 对外暴露的反向回调接口
func VerifySignatureMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			tsStr := r.Header.Get(HeaderTimestamp)
			nonce := r.Header.Get(HeaderNonce)
			sigReceived := r.Header.Get(HeaderSignature)

			if tsStr == "" || nonce == "" || sigReceived == "" {
				applog.C(ctx).Warnw("seamless_missing_signature_headers", logx.Field("path", r.URL.Path))
				http.Error(w, `{"code":"INVALID_SIGNATURE","message":"missing security headers"}`, http.StatusUnauthorized)
				return
			}

			// 1. 防重放校验 (时钟偏移判定)
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			now := time.Now().Unix()
			if err != nil || ts < now-MaxTimeSkewSec || ts > now+MaxTimeSkewSec {
				applog.C(ctx).Warnw("seamless_timestamp_skew_too_large",
					logx.Field("req_ts", ts),
					logx.Field("server_ts", now),
				)
				http.Error(w, `{"code":"REQUEST_EXPIRED","message":"timestamp skew exceeds threshold"}`, http.StatusUnauthorized)
				return
			}

			// 2. 读出 Body 并重新复原 (避免消费后下游 Handler 读不到)
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, `{"code":"BAD_REQUEST","message":"cannot read payload"}`, http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// 3. 复用 pkg/security 验签
			payload := security.BuildSignPayload(r.Method, r.URL.Path, string(bodyBytes), tsStr, nonce)
			if err := security.VerifySign(secret, payload, sigReceived); err != nil {
				applog.C(ctx).Errorw("seamless_signature_mismatch",
					logx.Field("received_sig", sigReceived),
					logx.Field("path", r.URL.Path),
				)
				http.Error(w, `{"code":"INVALID_SIGNATURE","message":"signature verification failed"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// FaultInjectionMiddleware 故障演练中间件 (用于本地测试断路器、死信队列与超时)
func FaultInjectionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. 模拟网络高延迟
			if latencyStr := r.Header.Get(HeaderMockLatency); latencyStr != "" {
				if ms, err := strconv.Atoi(latencyStr); err == nil && ms > 0 {
					applog.C(ctx).Infow("seamless_mock_latency_injected", logx.Field("ms", ms))
					time.Sleep(time.Duration(ms) * time.Millisecond)
				}
			}

			// 2. 模拟三方服务端故障
			switch r.Header.Get(HeaderMockFault) {
			case "504_GATEWAY_TIMEOUT":
				applog.C(ctx).Warnw("seamless_mock_fault_triggered_504")
				http.Error(w, `{"code":"GATEWAY_TIMEOUT","message":"Mock 504 Timeout"}`, http.StatusGatewayTimeout)
				return
			case "500_INTERNAL_ERROR":
				applog.C(ctx).Warnw("seamless_mock_fault_triggered_500")
				http.Error(w, `{"code":"INTERNAL_ERROR","message":"Mock 500 Error"}`, http.StatusInternalServerError)
				return
			case "HANG_TIMEOUT":
				// 故意挂起 6 秒，逼迫客户端触发 context 超时进入 DLQ
				time.Sleep(6 * time.Second)
				http.Error(w, `{"code":"DEADLINE_EXCEEDED"}`, http.StatusGatewayTimeout)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
