# FastGame 统一日志与全链路观测（Observability）技术规范

本文档定义 FastGame RGS 系统的统一结构化日志规范、全链路分布式追踪（Distributed Tracing）协议、数据安全脱敏底线与 ClickHouse 链路存储方案。

---

## 1. 架构总览

系统采用“结构化 JSON 日志（本地/标准输出收集） + 异步链路事件流（ClickHouse）”的双轨观测模型：

````text
[ Cocos Client ]
   |  (注入 X-Trace-Id / X-Client-Version)
   v
[ Edge Nginx / Gateway ]
   |  (补充 TraceParent，统一链路根 ID)
   v
[ RGS API (go-zero rest) ]
   |----> Seamless Wallet HTTP Client (透传 X-Trace-Id / X-Round-Id)
   |         `----> 捕获 Outbound 耗时、熔断状态与重试
   |
   |----> 本地结构化日志 (Stdout/File) ----> Vector / FluentBit ----> Loki / ELK
   |
   `----> Kafka (Header: trace_id, payload.traceId)
            |
            |----> Settle Consumer ----> ClickHouse "game_round_settled"
            |
            `----> Trace Consumer  ----> ClickHouse "trace_spans" (全景执行链)
````

- **底层引擎**：基于 go-zero 的 logx 组件（底层集成 zap），生产环境严格锁定为 JSON 编码。
- **核心功能包**：
    - `pkg/log`：统一标准字段、`log.C(ctx)` 链路上下文提取器、HTTP 与 Kafka 拦截中间件。
    - `pkg/trace`：TraceID/SpanID 编解码，向 ClickHouse 异步上报执行跨度（Span）。

---

## 2. 字段字典与命名约束

全系统（服务端、客户端、消费端）日志字段严格遵守 **snake_case** 小写下划线命名规范，严禁跨模块混用驼峰命名。

### 2.1 系统与链路追踪级字段（中间件自动注入）

| 字段名 | 类型 | 示例值 | 说明 |
|:---|:---|:---|:---|
| trace_id | string | c8f3e2a1b5d64f02a9b3c4d5e6f7a8b9 | 全链路唯一标识（32 位十六进制字符或 UUID） |
| span_id | string | 7b2a9e10d8c74b12 | 当前操作段或执行单元的 Span ID |
| service | string | rgs-api | 微服务组件代号（如 rgs-api, wallet-worker, admin-api） |
| env | string | prod | 运行环境（dev, staging, prod） |
| level | string | info | 日志级别：debug, info, warn, error, fatal |
| caller | string | turn/handler.go:88 | 触发日志的代码文件与行号 |
| duration_ms | int64 | 12 | 接口或子过程执行耗时（毫秒） |

### 2.2 博弈业务上下文核心字段（必须由 Context 携带）

| 字段名 | 类型 | 示例值 | 说明 |
|:---|:---|:---|:---|
| merchant_id | uint64 | 1001 | 商户主键 ID |
| merchant_code | string | M_LUCKY_01 | 商户唯一业务代号 |
| user_id | string | U99201938 | 下游商户系统的玩家唯一 ID |
| game_code | string | fishing_tycoon | 游戏机台全局代号 |
| round_id | string | R20260914001928 | **核心业务唯一主键**（单局下注业务流水号） |
| event | string | bet_settled | 事件标识（规范：动词_名词 或 领域_动作） |
| status | int | 200 | HTTP 响应状态码或内部业务状态码 |

---

## 3. 日志分级准则与数据安全脱敏

### 3.1 级别触发标准

| 日志级别 | 触发场景 | 告警策略 |
|:---|:---|:---|
| **DEBUG** | 离线算法验证、PRNG 初始种子运算细节、协议报文原始二进制 dump。 | 生产环境严格关闭。 |
| **INFO** | 单局正常扣款（Bet）、正常派彩（Win）、定时报表汇总、商户密钥轮换。 | 不告警，作为日常对账与排查轨迹依据。 |
| **WARN** | 三方无缝钱包耗时超过 1000ms、触发客户端限流（429）、Redis 锁竞争重试。 | 聚合进监控看板，不触发即时电话/消息呼叫。 |
| **ERROR** | 三方钱包 504 超时并入死信队列（DLQ）、单局对账差额、参数验签失败。 | **即时告警**（Telegram 运维群 / 钉钉机器人）。 |
| **FATAL** | 检测到 RTP 穿池疑似算法被刷、核心配置异常导致服务 Panic。 | **P0 级最高告警**，自动熔断机台并呼叫值班。 |

### 3.2 敏感数据脱敏红线（硬性合规要求）

> 严禁在日志中输出以下明文内容：

1. **商户密钥与凭证**：secret_key, api_key, private_key 严禁打印明文，只允许保留前缀（如 `sk_live_abc1***`）；
2. **算法种子明文**：未完成结算前的 server_seed 原文严禁落入控制台，仅允许记录其 server_seed_hash；
3. **玩家隐私凭据**：玩家真实姓名、银行卡号、手机号一律做掩码处理（如 `138****0000`）。

---

## 4. 服务端开发范式（Go-Zero）

### 4.1 服务启动初始化 (main.go)

````go
package main

import (
    "fastgame/pkg/log"
    "[github.com/zeromicro/go-zero/core/logx](https://github.com/zeromicro/go-zero/core/logx)"
)

func main() {
    // 根据环境自动匹配：生产环境默认切为 json 格式并启用 caller
    log.MustSetup("rgs-api", c.Log)
    defer logx.Close()

    // 启动服务流程...
}
````

### 4.2 业务上下文构建与规范日志输出

必须使用 `applog.C(ctx)` 获取携带完整链路 Trace 的 Logger，严禁使用未经上下文包装的原始打印方式。

````go
package logic

import (
    "context"
    "time"

    applog "fastgame/pkg/log"
    "fastgame/service/rgs/api/internal/types"
    "[github.com/zeromicro/go-zero/core/logx](https://github.com/zeromicro/go-zero/core/logx)"
)

func (l *GameTurnLogic) ProcessSpin(ctx context.Context, req *types.TurnReq) (*types.TurnResp, error) {
    // 1. 将业务核心元数据注入 Context
    ctx = applog.WithRoundContext(ctx, applog.RoundFields{
        MerchantCode: req.MerchantCode,
        GameCode:     req.GameCode,
        RoundId:      req.RoundId,
        UserId:       req.UserId,
    })

    // 2. 正常业务事件记录
    applog.C(ctx).Infow("bet_request_received",
        logx.Field("bet_amount", req.BetAmount),
        logx.Field("currency", req.Currency),
    )

    // 3. 外部网络调用链路度量
    startTime := time.Now()
    err := l.walletClient.DeductBet(ctx, req)
    duration := time.Since(startTime).Milliseconds()

    if err != nil {
        // 4. 异常捕获与死信跟踪
        applog.C(ctx).Errorw("seamless_wallet_bet_failed",
            logx.Field("error", err.Error()),
            logx.Field("duration_ms", duration),
            logx.Field("retry_scheduled", true),
        )
        return nil, err
    }

    applog.C(ctx).Infow("bet_settled_success",
        logx.Field("duration_ms", duration),
        logx.Field("payout", outcome.Payout),
        logx.Field("multiplier", outcome.Multiplier),
    )
    return resp, nil
}
````

### 4.3 跨协议传播（HTTP 与 Kafka）

- **HTTP 客户端对外调用（Outbound）**：
  必须向下游无缝钱包网关注入统一 Header：
  ````http
  X-Trace-Id: c8f3e2a1b5d64f02a9b3c4d5e6f7a8b9
  X-Round-Id: R20260914001928
  X-Provider: FastGame-RGS
  ````
- **Kafka 消息投递（Producer）**：
  发送前统一调用 `applog.InjectKafkaHeader(ctx, message)` 将 trace_id 写入 Kafka Header，同时消息体 JSON 顶层保留 traceId 字段。

---

## 5. 客户端日志接入规范（Cocos / WebGL）

客户端采用异常驱动上报机制，发生严重通信失败或动效渲染异常时进行静默回传。

````typescript
import { GameLogger } from '../util/GameLogger';

// 1. RgsClient 发起网络请求前自动注入 Trace
const traceId = GameLogger.getTraceId();

// 2. 发生严重运行时异常时上报
GameLogger.error("game_render_exception", {
    round_id: currentRoundId,
    game_code: "fishing_tycoon",
    error_stack: error.stack,
    device_info: {
        os: cc.sys.os,
        browser: cc.sys.browserType,
        render_system: cc.sys.renderType
    }
});
````

---

## 6. ClickHouse 链路追踪审计表 (trace_spans)

高价值注单执行链路通过异步 Kafka 写入 ClickHouse，支撑商户后台秒级调取单局执行瀑布流：

````sql
CREATE DATABASE IF NOT EXISTS rgs_analytics;

DROP TABLE IF EXISTS rgs_analytics.trace_spans;
CREATE TABLE rgs_analytics.trace_spans (
    `trace_id` String COMMENT '全链路唯一 TraceID',
    `span_id` String COMMENT '当前执行步骤 SpanID',
    `parent_span_id` String COMMENT '父级 SpanID (根节点为空)',
    `round_id` String COMMENT '关联单局号 (重要排查索引)',
    `merchant_code` LowCardinality(String) COMMENT '商户代号',
    `game_code` LowCardinality(String) COMMENT '游戏代号',
    `user_id` String COMMENT '玩家外部ID',
    
    `span_name` LowCardinality(String) COMMENT '动作名: wallet_bet, rng_math, wallet_win, ch_persist',
    `status_code` LowCardinality(String) COMMENT '状态: OK, TIMEOUT, ERROR',
    `duration_us` UInt32 COMMENT '执行耗时 (微秒)',
    `attributes` String COMMENT '结构化 JSON 属性 (如扣款金额、重试次数)',
    `error_message` String COMMENT '错误堆栈/原因',
    
    `start_time` DateTime64(6, 'UTC') COMMENT '步骤开始精确时间戳',
    `created_at` DateTime DEFAULT now() COMMENT '落盘时间'
)
ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(created_at)
PRIMARY KEY (merchant_code, game_code, created_at)
ORDER BY (merchant_code, game_code, created_at, round_id, trace_id, start_time)
SETTINGS index_granularity = 8192;
````

---

## 7. 运维配置与排查指引

### 7.1 服务配置文件模板 (etc/*.yaml)

````yaml
Log:
  ServiceName: "rgs-api"
  Mode: "file"              # 本地调试: console, 线上部署: file
  Path: "/var/log/fastgame" # 日志物理落盘路径
  Level: "info"             # 生产基线级别: info
  Compress: true            # 历史日志自动 gzip 压缩
  KeepDays: 7               # 本地磁盘保留天数 (过期清理或转存冷存储)
  StackCooldownMillis: 100
  Encoding: "json"          # 生产环境强制 json
````

### 7.2 环境变量强制覆盖规则

容器启动脚本会根据运行环境覆盖对应参数：

| 环境变量 | 匹配值 | 行为 |
|:---|:---|:---|
| FG_ENV | prod / production | 强制覆盖 Encoding 为 json，Level 锁定为 info[cite: 1] |
| GO_ENV | production | 等同于 FG_ENV=prod[cite: 1] |
| FG_LOG_LEVEL | debug / info / warn | 在线临时覆盖日志等级，免改配置文件重启 |

### 7.3 常见故障排查路径

1. **单局用户客诉排查：**
    - 获取玩家提供的 round_id；
    - 在商户 Admin 后台的“链路追踪”页面检索 round_id；
    - 提取全景瀑布流分析耗时：扣款耗时（ms） -> 算法推演耗时（us） -> 派彩耗时（ms）。
2. **三方钱包通信超时核验：**
    - 日志系统检索：`trace_id = "..." AND event = "seamless_wallet_bet_failed"`；
    - 提取 http_status 和原始响应报文，确认是否已落入 merchant_wallet_dlq 等待自愈。
3. **恶意并发脚本监控：**
    - 过滤分析：`event = "rate_limit_exceeded"`，按 user_id 和 client_ip 聚合统计 Top 攻击源。