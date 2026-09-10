# FastGame 日志系统

## 架构

```
Client (GameLogger) ──X-Trace-Id──► Nginx ──► RGS / Admin
                                         │
                                         ├──► Wallet HTTP (X-Trace-Id)
                                         └──► Kafka (header + JSON traceId)
                                                   │
                                                   ▼
                                            Consumer / Rollback
                                                   │
                                                   ▼
                                            ClickHouse trace_spans
```

- **引擎**：go-zero `logx`（底层 zap），生产环境 `Encoding: json`
- **关联包**：`pkg/log` — 统一字段名、`log.C(ctx)`、HTTP/Kafka 中间件
- **Trace**：`pkg/trace` 负责 ID 传播；`trace.Record` 写入 ClickHouse 步骤链

## 标准字段

| 字段 | 含义 |
|------|------|
| `service` | 服务名（全局） |
| `trace_id` | 全链路 ID |
| `round_id` | 局号 |
| `merchant_id` | 商户 |
| `user_id` | 玩家 |
| `event` | 事件名（Infow/Errorw 首参） |
| `duration_ms` | 耗时 |
| `status` | HTTP 状态码 |

## 用法

### Go 服务

```go
import applog "fastgame/pkg/log"

// worker 启动
applog.MustSetup("consumer", c.Log)

// 业务逻辑
ctx = applog.WithRound(ctx, roundID)
applog.C(ctx).Infow("bet_settled", logx.Field("win_minor", win))

// HTTP 中间件（RGS/Admin 已挂载）
server.Use(middleware.TraceMiddleware()) // → applog.HTTPMiddleware()
```

### 配置

各服务 `etc/*.yaml`：

```yaml
Log:
  ServiceName: rgs-api
  Mode: console      # prod: file
  Level: info
  Encoding: plain    # prod: json
```

### 客户端

```typescript
import { GameLogger } from '../util/GameLogger';
// RgsClient 自动捕获/回传 X-Trace-Id
GameLogger.error('bet_failed', { roundId, traceId: GameLogger.getTraceId() });
```

## 查询

- **实时日志**：按 `trace_id` 在 Loki/ELK 过滤（JSON 模式）
- **步骤链**：Admin → Trace 查询 → ClickHouse `trace_spans`
- **对账**：MySQL `pending_transactions.trace_id` + Kafka `traceId`

## 环境变量

| 变量 | 效果 |
|------|------|
| `FG_ENV=prod` | 未配置 Encoding 时默认 `json` |
| `GO_ENV=production` | 同上 |
