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
