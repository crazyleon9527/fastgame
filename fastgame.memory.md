# fastgame 项目记忆

## 项目构成
- `services/rgs` RGS 游戏服（go-zero REST，18888）；`services/admin` 后台（18889）；`services/consumer` 结算事件消费写 ClickHouse；`services/rollback` 钱包对账；`services/broadcast` 大奖 WebSocket（18890）。
- `internal/model` 手写 + goctl 生成混合；`pkg/*` 公共库；`engine/*` 新数学引擎（**尚未被任何服务使用**，只有测试引用）。
- 依赖：MySQL 8(13306) / Redis 7(16379) / Kafka(19092) / ClickHouse(19000/18123)；`docker compose` 起全部依赖，服务用 `bin\*.exe -f services\<svc>\etc\<svc>.yaml` 以隐藏窗口方式跑，日志在 `logs\<svc>.{out,err}.log`。

## 环境坑（重要）
- **绝不要用 PowerShell 的 `Get-Content`/`Set-Content` 往返含中文的源文件**：PS 5.1 按 GBK 读 UTF-8，会把中文变成乱码并吞掉换行（`pkg/async/runner.go` 就这么被毁过一次）。要改文件用编辑工具，或 `[System.IO.File]::ReadAllText/WriteAllText` + `UTF8Encoding($false)`。
- 含中文的 `.ps1` 必须存成 **UTF-8 BOM**，否则 PS 5.1 按 GBK 解析报错。
- `docker exec ... mysql` 的密码警告走 stderr，`$ErrorActionPreference='Stop'` 下会被当成错误 → 命令显示失败但实际已执行；判断结果请查库而不是看 exit code。
- `services/*/etc/` 同时有 `<svc>.yaml`（开发）和 `<svc>.prod.yaml`；`Get-ChildItem | Select -First 1` 会取到 prod，必须显式写文件名。
- Docker Desktop/WSL2 的 IPv6 端口转发是坏的：Windows 把 `sponge.localhost` 解析成 `::1`，后台走该域名会 502。用 `curl -H "Host: sponge.localhost" http://127.0.0.1:18000/admin/` 验证。
- 推 GitHub 需要走系统代理：`git -c http.proxy=http://127.0.0.1:19828 -c https.proxy=http://127.0.0.1:19828 push origin main`（直连 github.com:443 超时）。

## 账变体系（迁移 35/36，2026-09-20 落地）
- 三张表 + 一个收口，参考 platform-api 的 `transaction_type` / `transaction` / `CashService.AddTransaction`：
  - `transaction_types` 账变类型表：**变动方向是数据不是代码**。`io_type(IN/OUT)`、`balance_change(INCREASE/DECREASE/NONE)`、`frozen_change`。调用方只传 `TypeCode` + 正数金额，方向由表决定，所以调用点不可能写错符号。已种子：BET/WIN/REFUND/ROLLBACK/PROMO_CREDIT/ADJUST_ADD/ADJUST_SUB。
  - `game_transactions` 账变流水：每笔带 `balance_before/after` 快照 + `drift_minor`（不可解释差额）+ `external_tx_id`（钱包侧凭据号）+ `extra_data`。
  - `player_accounts` 影子账户：`balance_minor` 是**钱包返回值的镜像**，钱包才是权威。
  - `pkg/ledger.Poster`：唯一写入口。`Post`（自动开事务）/ `PostInTx`（并入调用方事务）。流程＝按 code 取类型 → `SELECT ... FOR UPDATE` 锁账户 → 记 before → 按类型算 after → 插流水（幂等判定）→ 改镜像 → 同事务登记 outbox 事件。
- 幂等键 `uk_merchant_round_type_status(merchant_id, round_id, tx_type, status)`。**必须带 status**：补偿场景下同一局的 WIN 先落 `PENDING_RETRY`（钱没动）再落 `SUCCESS`（钱到账），少了 status 第二条会被当重复丢掉，变成"钱到了账上没有"。人工调账 `round_id` 为 NULL，不受该键约束。
- 关键语义：**钱包返回了余额时以钱包为准，本地算成负数不拒绝、而是记 drift**。镜像为空（玩家首次出现）时若按"余额不足"拒绝，这笔真实资金变动就从账本上消失了。只有"既没钱包余额、镜像也不足"（如人工调账）才拒绝。
- 接入点：RGS 的 `wallet.Bet` 成功后（独立事务，`postBet`）、`recordSettled` 事务内的派彩 WIN、派彩失败时记 `PENDING_RETRY`；rollback 的 `Reconciler`/`OrphanReconciler` 补偿成功后补记 SUCCESS。
- 后台两个接口（`services/admin`，与其它写接口同组：AuthMiddleware + TotpGateMiddleware + JWT）：
  - `POST /api/v1/admin/ledger/adjustments` 人工调账：登记"钱包侧已做过的人工调整"，`externalTxId`/`remark` 必填，`WalletBalance=nil` 时按类型推算镜像；操作人 id/用户名/IP 写进 `extra_data`；**只允许 admin 角色**（`pkg/auth/rbac.go` 的 `operatorCanWrite` 按 `/ledger/adjustments` 拦截）。已知未做：同一 `externalTxId` 重复提交会落两条流水。
  - `GET /api/v1/admin/ledger/transactions` 流水查询：支持 `merchantId/userId/txType/roundId/status/driftOnly/startTime/endTime` 分页；`typeName` 用 `ListEnabledTypes` 一次查表映射，不做 N+1。
  - admin 的 `svc.ServiceContext` 用 `Ledger`（sink 传 nil）、`LedgerModel`、`DB sqlx.SqlConn`（字段名对齐 RGS）。
- 账变事件 topic `game.ledger.posted`（`pkg/ledger.OutboxSink`）。目前只有 RGS 跑 outbox dispatcher 所以只有它配了 sink；admin/rollback 传 nil，等接入结算/对账时再补 dispatcher。
- 排查入口：`SELECT * FROM game_transactions WHERE drift_minor <> 0`、`player_accounts.drift_count`。**dev 环境跨服务补偿必然报 drift**——mock 钱包是进程内独立状态（`pkg/wallet/mock.go` 的 `balances` map 每进程一份），生产用共享钱包服务时才有意义。
- `internal/model/biz/` 下 31 个 goctl 模型里 **27 个没有对应的表**（`game_transactions` 曾在其列，现已在 `internal/model/ledger.go` 手写落地）。其余（`merchant_contracts` GGR/NGR 结算模式、`merchant_financial_periods` 账单、`merchant_reconciliation_diffs` 对账差异、`promo_*` 等）仍未落地，是后续结算/对账阶段的现成设计。

## 数据库与迁移
- 迁移文件放 `docker/mysql/init/NN-<name>-migration.sql`，用 `scripts\apply-migration.ps1 <版本号前缀>` 应用（内部 `docker cp` + `mysql -e "source ..."`，避免 Windows 管道编码问题）。已应用的迁移**不再修改**，续作用新编号文件。
- ClickHouse 迁移放 `docker/clickhouse/init/`，用 `scripts\apply_clickhouse_migration.ps1`（`--multiquery --queries-file`）。**绝不能把多条语句管道进 clickhouse-client**：曾因此只执行了 RENAME/DROP 就截断，把 `game_round_settled` 等表打没了。改表结构一律「staging 新表 → 拷数据 → RENAME」。
- 三张对账表 `pending_transactions` / `game_round_replay` / `wallet_pending_ops` 的唯一键以 `merchant_id` 前导（33 号迁移）；`round_id` 只在商户内唯一，因此**按 round_id 定位行必须带 merchant_id**，`merchantID=0` 仅用于只读排障（走 34 号迁移补的 `idx_round_id`）。写路径带 0 会串改别家商户的数据。
- 模型里 Go 声明为 `string` 的 ID 列，库里必须是字符型（有测试 `TestSchemaStringIDColumnsAreVarchar` 守着）。
- 金额统一 `pkg/money`，scale=10000 的 minor units。

## 已知未修问题（按严重程度）
1. ~~RGS 结算用 864% RTP 的旧赔付表~~ **已修（2026-09-20）**：赔付表统一到 `pkg/par.Default96`（精确 96%），RGS 结算（`pkg/prng.outcomeFromRoll`）、`engine/games/fishing` 插件、客户端验算 `web/shared/prng-money.js` 三处共用同一张表；插件改成由 provably-fair 种子确定性推演（原先是 `crypto/rand`，无法回放/验证）。`pkg/prng/js_parity_test.go` 会真调 node 逐局比对客户端与服务端口径。
2. ~~`blacklist:user_id:*` 孤儿键永久封禁~~ **已修**：`security.Blacklist.Reconcile` 以库为准做全量对账（含删除库中已不存在的键），RGS/后台启动时执行。
3. watchdog 误报 **已修**：原实现 10 局 + $100 就判定，正常波动（10 局里一个 20x）就会超过 180% 阈值 → flagged 游戏 + 封禁玩家 + 全部下注 403。现 `MinSamples` 默认 2000（按 6σ 上界推导）、告警冷却 10 分钟、自动封禁必须带过期时间（原来写的是永久封禁）。
4. `pkg/prng` 把明文 `server_seed` 随每次下注返回；`GET /api/v1/game/replay/:roundId` 无需鉴权（现已支持可选 `merchantId` 限定商户）。
5. `audit_logs` 从未写入；RBAC 缺失角色时回退 `admin`；TOTP 开关硬编码关闭。
6. wallet 慢响应按失败处理但不落补偿记录（`pkg/wallet/breaker.go`）；锁 TTL 3s < 钱包超时 5s。
7. `engine.UniversalHost` / `engine/turn_logic.go` 仍是死代码（没有服务构造它）；RGS 走的是 `pkg/prng` + `pkg/par`。若将来要接插件路径，注意别退回 `crypto/rand`。

## 约定
- 后台/模型/文档注释一律**简体中文**。
- 提交信息用中文，说明「为什么」而不只是「改了什么」。
- `web/admin/` 的构建产物（index.html + static/*）**不提交**。
- 后台 goroutine 一律用 `pkg/async.Runner`（panic 兜底 + 并发闸门 + 可 Drain + trace 继承），不要再写裸 `go func()`。
- 链路 ID 原语在 `pkg/traceid`（叶子包），`pkg/trace` 转发；这是为了让 `pkg/async` 能继承 trace 而不与 `pkg/trace` 形成 import 环。
