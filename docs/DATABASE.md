# FastGame 数据库设计

> 多商户 · 自研多游戏 · MySQL 8.0 · utf8mb4 · UTC
>
> **本文档由 `scripts/gen_database_doc.ps1` 从运行中的 MySQL 元数据生成，请勿手工编辑。**
>
> 重新生成：`powershell -ExecutionPolicy Bypass -File .\scripts\gen_database_doc.ps1`

**当前规模：41 张基表 + 2 个视图**（information_schema 实测）

---

## 一、设计原则

1. **平台层 vs 商户层**：`games` / `game_categories` / `game_rtp_tiers` 是平台统一维护的游戏目录；商户通过 `merchant_games` 开通并覆盖参数。
2. **注单明细不落 MySQL**：海量注单在 ClickHouse（`game_round_settled`）。MySQL 只存配置、会话、对账与审计，因此 `game_round_replay` 只保留确定性回放所需的种子，不存派彩金额。
3. **金额一律 minor units（BIGINT）**：与 `pkg/money` 的 Scale=10000 对齐（4 位小数）。`08-money-migration.sql` 已把所有金额列从 DECIMAL 迁为 BIGINT。
4. **RTP 用 PPM**：96% = 960000（parts per million），避免浮点。
5. **状态列用 tinyint/varchar + CHECK 约束**：`22-status-checks-migration.sql` 引入。
6. **时间统一 UTC，精度 datetime(3)**：由 MySQL 容器 --default-time-zone=+00:00 保证。
7. **迁移可重复执行**：新脚本使用 CREATE TABLE IF NOT EXISTS / 条件 ALTER，并登记到 `schema_migrations`。

## 二、字段与命名约定

| 后缀 / 前缀 | 含义 |
|---|---|
| `*_minor` | 金额，BIGINT minor units（1.0 = 10000） |
| `*_ppm` | 比率，百万分之一 |
| `merchant_code` | 商户业务编码（字符串，对外使用） |
| `merchant_id` | 商户主键（对应 merchants.id，内部关联） |
| `game_code` | 游戏业务编码（对应 games.game_code） |
| `game_id` | 游戏主键（对应 games.id） |
| `round_id` | 单局唯一 ID，同时作为 provably-fair 的 nonce |
| `status` | 通常 1=启用 / 0=停用；games 为 0/1/2 三态 |
| `created_at` / `updated_at` | datetime(3)；日志类表用 timestamp(3) |

## 三、全表清单（按业务域）

### A. 商户与钱包

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `merchants` | 0 | `merchantsModel_gen.go` | 商户主体 |
| `merchant_wallet_configs` | 0 | **缺失** | 商户钱包配置 |
| `merchant_currencies` | 0 | **缺失** | 商户币种 |
| `currencies` | 3 | **缺失** | 币种主数据 |
| `commission_rules` | 1 | **缺失** | 商户分润规则 |

### B. 游戏目录与配置

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `games` | 3 | **缺失** | 平台游戏目录 |
| `game_categories` | 4 | **缺失** | 游戏分类 |
| `game_rtp_tiers` | 5 | **缺失** | 游戏 RTP 档位 |
| `merchant_games` | 3 | **缺失** | 商户游戏开通 |
| `game_configs` | 2 | `gameConfigsModel_gen.go` | 游戏参数配置 |
| `game_client_versions` | 3 | **缺失** | 游戏客户端版本发布 |
| `merchant_game_versions` | 0 | **缺失** | 商户游戏版本策略 |

### C. 结算与对账

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `settlement_periods` | 0 | `settlementPeriodsModel.go` | 商户结算周期账单 |
| `merchant_settlement_lines` | 0 | **缺失** | 结算周期明细行 |
| `daily_settlements` | 0 | `dailySettlementsModel.go` | 每日结算对账单 |
| `pending_transactions` | 0 | `pendingTransactionsModel.go` | 孤儿注单自动对账补偿 |
| `wallet_pending_ops` | 0 | `walletPendingOpsModel.go` | 钱包待对账 |
| `game_round_replay` | 0 | `gameRoundReplayModel.go` | 确定性回放输入 |

### D. 风控

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `risk_alerts` | 0 | `riskAlertsModel.go` | RTP 风控告警 |
| `risk_blacklist` | 0 | `riskBlacklistModel.go` | 风控黑名单 |
| `player_merchant_profiles` | 0 | **缺失** | 商户下玩家档案 |

### E. 会话、审计与运维

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `game_sessions` | 0 | **缺失** | 玩家会话登记 |
| `audit_logs` | 0 | `auditLogsModel.go` | 后台操作审计日志 |
| `game_maintenance_windows` | 0 | **缺失** | 游戏维护窗口 |
| `api_rate_limits` | 0 | **缺失** | 商户 API 限流 |
| `merchant_webhooks` | 0 | **缺失** | 商户出站 Webhook |

### F. 多语言（i18n）

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `locales` | 7 | **缺失** | 平台支持语言 |
| `i18n_bundles` | 4 | **缺失** | i18n 词典命名空间 |
| `i18n_messages` | 33 | **缺失** | i18n 词典键 |
| `i18n_message_translations` | 33 | **缺失** | i18n 词典译文 |
| `i18n_entity_translations` | 17 | **缺失** | 业务实体字段翻译 |
| `merchant_locales` | 3 | **缺失** | 商户启用语言 |

### G. 后台权限

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `roles` | 3 | `rolesModel.go` | 角色 |
| `admin_users` | 2 | `adminAuthModel.go`, `adminUsersModel_gen.go` | 管理后台用户 |

### H. 元数据与归档

| 表 | 行数 | 模型文件 | 表注释 |
|---|---:|---|---|
| `schema_migrations` | 18 | **缺失** | 数据库迁移登记 |
| `schema_archive_policies` | 3 | **缺失** | 表归档保留策略 |

> 行数为 InnoDB 统计估算值，仅供参考。「模型文件」指 `internal/model/` 下的对应文件。

## 四、表结构明细

### 金额列速查

以下列均为 BIGINT minor units（Scale=10000）：

`commission_rules.min_fee_minor` · `daily_settlements.total_bet` · `daily_settlements.total_win` · `game_round_replay.bet_amount` · `game_transactions.drift_minor` · `games.min_bet_minor` · `games.max_bet_minor` · `merchant_games.min_bet_minor` · `merchant_games.max_bet_minor` · `merchant_settlement_lines.total_bet_minor` · `merchant_settlement_lines.total_win_minor` · `merchant_settlement_lines.commission_minor` · `merchant_settlement_lines.amount_minor` · `pending_transactions.bet_amount` · `pending_transactions.win_amount` · `player_accounts.balance_minor` · `player_accounts.frozen_minor` · `player_accounts.last_drift_minor` · `player_merchant_profiles.lifetime_bet_minor` · `player_merchant_profiles.lifetime_win_minor` · `risk_alerts.total_bet` · `risk_alerts.total_win` · `settlement_periods.total_bet_minor` · `settlement_periods.total_win_minor` · `settlement_periods.ggr_minor` · `settlement_periods.commission_minor` · `settlement_periods.net_payable_minor` · `v_merchant_game_lobby.merchant_min_bet_minor` · `v_merchant_game_lobby.merchant_max_bet_minor` · `v_merchant_game_lobby.game_min_bet_minor` · `v_merchant_game_lobby.game_max_bet_minor` · `v_merchant_game_lobby.effective_min_bet_minor` · `v_merchant_game_lobby.effective_max_bet_minor` · `v_merchant_game_lobby_i18n.effective_min_bet_minor` · `v_merchant_game_lobby_i18n.effective_max_bet_minor` · `wallet_pending_ops.bet_amount` · `wallet_pending_ops.win_amount`

---

### A. 商户与钱包

#### `merchants`

> 商户主体

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_code` | varchar(64) | 否 | - | UK | 商户唯一编码 |
| `name` | varchar(128) | 否 | - |  | 商户名称 |
| `public_key` | text | 是 | - |  | API 公钥 |
| `private_key` | text | 是 | - |  | API 私钥 (加密存储) |
| `private_key_prev` | text | 是 | - |  | 轮换过渡期旧私钥 |
| `private_key_prev_expires_at` | datetime(3) | 是 | - |  | 旧私钥失效时间 |
| `allowed_ips` | json | 是 | - |  | 聚合器报备公网 IP 白名单 |
| `status` | tinyint | 否 | `1` | IDX | 1=启用 0=禁用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_status`（普通）：`status`
- `PRIMARY`（主键）：`id`
- `uk_merchant_code`（唯一）：`merchant_code`

#### `merchant_wallet_configs`

> 商户钱包配置

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | UK | 商户 ID（merchants.id） |
| `wallet_type` | varchar(32) | 否 | `seamless` |  | 钱包类型：seamless/transfer/mock |
| `base_url` | varchar(512) | 否 | - |  | 接口基础地址 |
| `api_key` | varchar(256) | 是 | - |  | API 密钥 |
| `sign_secret` | text | 是 | - |  | HMAC 密钥，建议加密存储 |
| `sign_enabled` | tinyint | 否 | `1` |  | 1=启用签名 0=不签名 |
| `verify_response` | tinyint | 否 | `1` |  | 1=校验响应签名 0=不校验 |
| `timeout_ms` | int | 否 | `5000` |  | 超时毫秒数 |
| `mock` | tinyint | 否 | `0` |  | 1=Mock 钱包 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `PRIMARY`（主键）：`id`
- `uk_merchant_wallet`（唯一）：`merchant_id`

#### `merchant_currencies`

> 商户币种

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `currency_code` | char(8) | 否 | - |  | 币种编码（currencies.code） |
| `is_default` | tinyint | 否 | `0` |  | 1=默认币种 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_default`（普通）：`merchant_id`, `is_default`
- `PRIMARY`（主键）：`id`
- `uk_merchant_currency`（唯一）：`merchant_id`, `currency_code`

#### `currencies`

> 币种主数据

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `code` | char(8) | 否 | - | PK | ISO 或内部码，如 USD CNY USDT |
| `name` | varchar(64) | 否 | - |  | 名称 |
| `symbol` | varchar(8) | 是 | - |  | 货币符号 |
| `minor_units` | tinyint | 否 | `4` |  | 小数位数，对应 money.Scale=10000 时为 4 |
| `status` | tinyint | 否 | `1` |  | 1=启用 0=禁用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `PRIMARY`（主键）：`code`

#### `commission_rules`

> 商户分润规则

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `game_id` | bigint unsigned | 是 | - | IDX | NULL=全部游戏 |
| `rule_type` | varchar(32) | 否 | `ggr_share` |  | 分润方式：ggr_share/fixed_fee |
| `rate_ppm` | bigint | 否 | - |  | 150000=15% 分成 |
| `min_fee_minor` | bigint | 否 | `0` |  | 最低手续费，minor units |
| `effective_from` | date | 否 | - |  | 生效起始时间（UTC） |
| `effective_to` | date | 是 | - |  | NULL=长期有效 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_game_id`（普通）：`game_id`
- `idx_merchant_effective`（普通）：`merchant_id`, `effective_from`, `status`
- `idx_merchant_game_status`（普通）：`merchant_id`, `game_id`, `status`, `effective_from`
- `PRIMARY`（主键）：`id`

### B. 游戏目录与配置

#### `games`

> 平台游戏目录

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `game_code` | varchar(64) | 否 | - | UK | 全局唯一游戏标识 |
| `name` | varchar(128) | 否 | - |  | 名称 |
| `category_id` | bigint unsigned | 是 | - | IDX | 分类 ID（game_categories.id） |
| `game_type` | varchar(32) | 否 | - | IDX | 游戏类型：fishing/slot/crash/table |
| `default_rtp_ppm` | bigint | 否 | `960000` |  | 默认 RTP，96%=960000 |
| `volatility` | varchar(16) | 是 | - |  | 波动性：low/medium/high |
| `min_bet_minor` | bigint | 否 | `10000` |  | 默认最小注 minor units |
| `max_bet_minor` | bigint | 否 | `10000000` |  | 默认最大注 minor units |
| `client_version` | varchar(32) | 是 | - |  | 当前客户端版本 |
| `thumbnail_url` | varchar(512) | 是 | - |  | 缩略图地址 |
| `description` | text | 是 | - |  | 说明 |
| `status` | tinyint | 否 | `1` |  | 1=上架 0=下架 2=维护 |
| `launched_at` | datetime(3) | 是 | - |  | 上线时间（UTC） |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_category_status`（普通）：`category_id`, `status`
- `idx_game_type`（普通）：`game_type`
- `idx_type_status_launched`（普通）：`game_type`, `status`, `launched_at`
- `PRIMARY`（主键）：`id`
- `uk_game_code`（唯一）：`game_code`

#### `game_categories`

> 游戏分类

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `code` | varchar(32) | 否 | - | UK | 分类编码：fishing/slot/crash/table |
| `name` | varchar(64) | 否 | - |  | 名称 |
| `sort_order` | int | 否 | `0` |  | 排序序号 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `PRIMARY`（主键）：`id`
- `uk_category_code`（唯一）：`code`

#### `game_rtp_tiers`

> 游戏 RTP 档位

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `game_id` | bigint unsigned | 否 | - | IDX | 游戏 ID（games.id） |
| `tier_code` | varchar(32) | 否 | - |  | default/high/low 等 |
| `target_rtp_ppm` | bigint | 否 | - |  | 目标 RTP ppm |
| `par_sheet_ref` | varchar(128) | 是 | - |  | PAR 表文件/版本引用 |
| `weight` | int | 否 | `100` |  | 随机权重 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_game_id`（普通）：`game_id`
- `PRIMARY`（主键）：`id`
- `uk_game_tier`（唯一）：`game_id`, `tier_code`

#### `merchant_games`

> 商户游戏开通

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `game_id` | bigint unsigned | 否 | - | IDX | 游戏 ID（games.id） |
| `rtp_tier_code` | varchar(32) | 是 | - |  | 覆盖默认 RTP 档位 |
| `min_bet_minor` | bigint | 是 | - |  | 覆盖最小注，NULL=用游戏默认 |
| `max_bet_minor` | bigint | 是 | - |  | 覆盖最大注 |
| `sort_order` | int | 否 | `0` |  | 商户大厅排序 |
| `status` | tinyint | 否 | `1` |  | 1=开通 0=关闭 |
| `opened_at` | datetime(3) | 是 | - |  | 开通时间 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_game_id`（普通）：`game_id`
- `idx_game_status_sort`（普通）：`game_id`, `status`, `sort_order`
- `idx_merchant_status`（普通）：`merchant_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_merchant_game`（唯一）：`merchant_id`, `game_id`

#### `game_configs`

> 游戏参数配置

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `game_code` | varchar(64) | 否 | - |  | 游戏标识 |
| `config_key` | varchar(128) | 否 | - |  | 配置键 |
| `config_value` | json | 否 | - |  | 配置值 |
| `rtp_tier` | varchar(32) | 是 | - |  | RTP 档位 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_game`（普通）：`merchant_id`, `game_code`
- `idx_merchant_status`（普通）：`merchant_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_merchant_game_key`（唯一）：`merchant_id`, `game_code`, `config_key`

#### `game_client_versions`

> 游戏客户端版本发布

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `game_id` | bigint unsigned | 否 | - | IDX | 游戏 ID（games.id） |
| `version` | varchar(32) | 否 | - |  | 语义化版本，如 1.2.0 |
| `bundle_url` | varchar(512) | 是 | - |  | CDN 路径或产物地址 |
| `bundle_hash` | char(64) | 是 | - |  | 包体 SHA-256 |
| `changelog` | text | 是 | - |  | 更新日志 |
| `min_engine_version` | varchar(32) | 是 | - |  | 要求的引擎版本（Cocos） |
| `status` | varchar(16) | 否 | `draft` |  | 状态：draft/published/deprecated |
| `published_at` | datetime(3) | 是 | - |  | 发布时间（UTC） |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_game_published`（普通）：`game_id`, `status`, `published_at`
- `idx_game_status`（普通）：`game_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_game_version`（唯一）：`game_id`, `version`

#### `merchant_game_versions`

> 商户游戏版本策略

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `game_id` | bigint unsigned | 否 | - |  | 游戏 ID（games.id） |
| `client_version_id` | bigint unsigned | 否 | - |  | 客户端版本 ID（game_client_versions.id） |
| `min_version_id` | bigint unsigned | 是 | - |  | 低于此版本强制升级 |
| `force_upgrade` | tinyint | 否 | `0` |  | 1=强制升级 0=可选升级 |
| `rollout_percent` | tinyint | 否 | `100` |  | 灰度发布比例 0-100 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `effective_from` | datetime(3) | 是 | - |  | 生效起始时间（UTC） |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_game`（普通）：`merchant_id`, `game_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_merchant_game_version`（唯一）：`merchant_id`, `game_id`, `client_version_id`

### C. 结算与对账

#### `settlement_periods`

> 商户结算周期账单

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `period_type` | varchar(16) | 否 | `weekly` |  | 周期类型：daily/weekly/monthly |
| `period_start` | date | 否 | - | IDX | 结算周期开始（UTC） |
| `period_end` | date | 否 | - |  | 结算周期结束（UTC） |
| `currency_code` | char(8) | 否 | `USD` |  | 币种编码（currencies.code） |
| `total_bet_minor` | bigint | 否 | `0` |  | 总下注额，minor units |
| `total_win_minor` | bigint | 否 | `0` |  | 总派彩额，minor units |
| `ggr_minor` | bigint | 否 | `0` |  | GGR = 总下注 − 总派彩 |
| `commission_minor` | bigint | 否 | `0` |  | 平台从 GGR 中抽取的分成 |
| `net_payable_minor` | bigint | 否 | `0` |  | 应付净额（平台应收） |
| `total_rounds` | bigint unsigned | 否 | `0` |  | 总局数 |
| `status` | varchar(16) | 否 | `draft` | IDX | 状态：draft/pending_review/confirmed/invoiced/paid |
| `confirmed_by` | bigint unsigned | 是 | - |  | 确认人（admin_users.id） |
| `confirmed_at` | datetime(3) | 是 | - |  | 确认时间（UTC） |
| `notes` | varchar(512) | 是 | - |  | 备注 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_status`（普通）：`merchant_id`, `status`
- `idx_period_range`（普通）：`period_start`, `period_end`
- `idx_status_period`（普通）：`status`, `period_end`
- `PRIMARY`（主键）：`id`
- `uk_merchant_period`（唯一）：`merchant_id`, `period_type`, `period_start`, `period_end`

#### `merchant_settlement_lines`

> 结算周期明细行

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `settlement_period_id` | bigint unsigned | 否 | - | IDX | 结算周期 ID（settlement_periods.id） |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `line_type` | varchar(32) | 否 | `daily_aggregate` |  | 明细类型：daily_aggregate/game_breakdown/commission/adjustment |
| `ref_date` | date | 是 | - |  | daily_aggregate 类型对应的日期 |
| `game_id` | bigint unsigned | 是 | - | IDX | 游戏 ID（games.id） |
| `daily_settlement_id` | bigint unsigned | 是 | - | IDX | 关联 daily_settlements.id |
| `description` | varchar(256) | 是 | - |  | 说明 |
| `total_bet_minor` | bigint | 否 | `0` |  | 总下注额，minor units |
| `total_win_minor` | bigint | 否 | `0` |  | 总派彩额，minor units |
| `total_rounds` | bigint unsigned | 否 | `0` |  | 总局数 |
| `commission_rate_ppm` | bigint | 是 | - |  | 结算时的费率快照 |
| `commission_minor` | bigint | 否 | `0` |  | 平台分成额，minor units |
| `amount_minor` | bigint | 否 | `0` |  | 带符号的明细金额 |
| `sort_order` | int | 否 | `0` |  | 排序序号 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |

**索引**

- `idx_daily_settlement`（普通）：`daily_settlement_id`
- `idx_game_id`（普通）：`game_id`
- `idx_merchant_date`（普通）：`merchant_id`, `ref_date`
- `idx_period_sort`（普通）：`settlement_period_id`, `sort_order`
- `PRIMARY`（主键）：`id`

#### `daily_settlements`

> 每日结算对账单

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `settle_date` | date | 否 | - |  | 结算日期 UTC |
| `total_bet` | bigint | 否 | `0` |  | 总下注额，minor units |
| `total_win` | bigint | 否 | `0` |  | 总派彩额，minor units |
| `total_rounds` | bigint unsigned | 否 | `0` |  | 总局数 |
| `status` | tinyint | 否 | `0` |  | 0=待确认 1=已确认 |
| `settlement_period_id` | bigint unsigned | 是 | - | IDX | 汇总进结算周期后的关联 ID（settlement_periods.id） |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_date_status`（普通）：`merchant_id`, `settle_date`, `status`
- `idx_settlement_period`（普通）：`settlement_period_id`
- `PRIMARY`（主键）：`id`
- `uk_merchant_date`（唯一）：`merchant_id`, `settle_date`

#### `pending_transactions`

> 孤儿注单自动对账补偿

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `trace_id` | varchar(64) | 否 | - | IDX | 全链路 TraceID |
| `round_id` | varchar(64) | 否 | - | IDX | 局 ID，同时作为 nonce |
| `merchant_id` | bigint unsigned | 否 | `0` | IDX | 商户 ID（merchants.id），0=未归属 |
| `merchant_code` | varchar(32) | 否 | - |  | 商户编码 |
| `user_id` | varchar(64) | 否 | - |  | 下游玩家唯一ID |
| `game_code` | varchar(32) | 否 | - |  | 游戏编码 |
| `phase` | varchar(32) | 否 | - |  | 阶段：bet_debited/win_pending/settled/orphan |
| `status` | varchar(16) | 否 | `pending` | IDX | 状态：pending/done/failed |
| `bet_amount` | bigint | 否 | - |  | 下注额，minor units（scale=10000） |
| `win_amount` | bigint | 否 | `0` |  | 派彩额，minor units |
| `expected_action` | varchar(16) | 否 | - |  | 期望补偿动作：settle_win/rollback_bet/none |
| `wallet_bet_status` | varchar(16) | 否 | `confirmed` |  | 钱包扣款状态 |
| `wallet_win_status` | varchar(16) | 否 | `unknown` |  | 钱包派彩状态 |
| `retry_count` | int | 否 | `0` |  | 已重试次数 |
| `last_error` | varchar(512) | 是 | - |  | 最近一次错误信息 |
| `created_at` | timestamp(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | timestamp(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_id_status`（普通）：`merchant_id`, `status`, `updated_at`
- `idx_round_id`（普通）：`round_id`
- `idx_status_created`（普通）：`status`, `created_at`
- `idx_status_updated`（普通）：`status`, `updated_at`
- `idx_trace_id`（普通）：`trace_id`
- `PRIMARY`（主键）：`id`
- `uk_merchant_round`（唯一）：`merchant_id`, `round_id`

#### `wallet_pending_ops`

> 钱包待对账

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `round_id` | varchar(128) | 否 | - | IDX | 局 ID，同时作为 nonce |
| `merchant_id` | bigint unsigned | 否 | `0` | IDX | 商户 ID（merchants.id），0=未归属 |
| `merchant_code` | varchar(64) | 否 | - | IDX | 商户编码 |
| `user_id` | varchar(64) | 否 | - |  | 下游玩家唯一ID |
| `op_type` | varchar(32) | 否 | - |  | 操作类型：win_failed / win_timeout / rollback |
| `bet_amount` | bigint | 否 | `0` |  | 下注额，minor units |
| `win_amount` | bigint | 否 | `0` |  | 派彩额，minor units |
| `status` | varchar(16) | 否 | `pending` | IDX | 状态：pending / done / failed |
| `retry_count` | int | 否 | `0` |  | 已重试次数 |
| `last_error` | text | 是 | - |  | 最近一次错误信息 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_id_status`（普通）：`merchant_id`, `status`, `updated_at`
- `idx_merchant_status`（普通）：`merchant_code`, `status`, `updated_at`
- `idx_round_id`（普通）：`round_id`
- `idx_status_updated`（普通）：`status`, `updated_at`
- `PRIMARY`（主键）：`id`
- `uk_merchant_round_op`（唯一）：`merchant_id`, `round_id`, `op_type`

#### `game_round_replay`

> 确定性回放输入

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `round_id` | varchar(64) | 否 | - | IDX | 局 ID，同时作为 nonce |
| `merchant_id` | bigint unsigned | 否 | `0` | IDX | 商户 ID（merchants.id），0=未归属 |
| `merchant_code` | varchar(32) | 否 | - | IDX | 商户编码 |
| `user_id` | varchar(64) | 否 | - | IDX | 下游玩家唯一ID |
| `game_code` | varchar(32) | 否 | - | IDX | 游戏编码 |
| `server_seed` | varchar(128) | 否 | - |  | 服务端种子（hex） |
| `client_seed` | varchar(128) | 否 | - |  | 客户端种子 |
| `nonce` | varchar(64) | 否 | - |  | 局 ID（provably-fair 的 nonce） |
| `bet_amount` | bigint | 否 | - |  | 下注额，minor units |
| `sequence_id` | bigint unsigned | 否 | `0` |  | 会话内单调递增序号 |
| `created_at` | timestamp | 否 | `CURRENT_TIMESTAMP` |  | 创建时间（UTC） |

**索引**

- `idx_game_created`（普通）：`game_code`, `created_at`
- `idx_merchant_created`（普通）：`merchant_code`, `created_at`
- `idx_merchant_id_created`（普通）：`merchant_id`, `created_at`
- `idx_round_id`（普通）：`round_id`
- `idx_user_created`（普通）：`user_id`, `created_at`
- `PRIMARY`（主键）：`id`
- `uk_merchant_round`（唯一）：`merchant_id`, `round_id`

### D. 风控

#### `risk_alerts`

> RTP 风控告警

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 是 | - | IDX | 商户 ID（merchants.id） |
| `alert_type` | varchar(32) | 否 | - |  | 告警类型 |
| `scope_type` | varchar(16) | 否 | - |  | 作用域：user 或 game |
| `scope_value` | varchar(128) | 否 | - |  | 作用域取值 |
| `merchant_code` | varchar(32) | 否 | - | IDX | 商户编码 |
| `game_code` | varchar(32) | 否 | - |  | 游戏编码 |
| `rtp_ppm` | bigint | 否 | - |  | 实际RTP * 1e6，180%=1800000 |
| `total_bet` | bigint | 否 | - |  | 总下注额，minor units |
| `total_win` | bigint | 否 | - |  | 总派彩额，minor units |
| `sample_size` | bigint | 否 | - |  | 样本局数 |
| `action_taken` | varchar(64) | 否 | - |  | 已执行的处置动作 |
| `status` | varchar(16) | 否 | `open` | IDX | 状态：1=启用 0=停用 |
| `created_at` | timestamp(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |

**索引**

- `idx_merchant_created`（普通）：`merchant_code`, `created_at`
- `idx_merchant_id_created`（普通）：`merchant_id`, `created_at`
- `idx_status_created`（普通）：`status`, `created_at`
- `PRIMARY`（主键）：`id`

#### `risk_blacklist`

> 风控黑名单

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `list_type` | varchar(32) | 否 | - | IDX | 名单类型：ip / user_id / merchant |
| `list_value` | varchar(128) | 否 | - |  | 黑名单值 |
| `reason` | varchar(256) | 是 | - |  | 封禁原因 |
| `status` | tinyint | 否 | `1` | IDX | 1=生效 0=解除 |
| `expires_at` | datetime(3) | 是 | - |  | NULL=永久 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_status_expires`（普通）：`status`, `expires_at`
- `PRIMARY`（主键）：`id`
- `uk_type_value`（唯一）：`list_type`, `list_value`

#### `player_merchant_profiles`

> 商户下玩家档案

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `user_id` | bigint unsigned | 否 | - |  | 商户体系内的玩家 ID |
| `external_user_ref` | varchar(128) | 是 | - |  | 商户侧的玩家字符串 ID（可选） |
| `display_name` | varchar(128) | 是 | - |  | 显示名称 |
| `vip_level` | tinyint | 否 | `0` |  | VIP 等级：0=普通，越大越高 |
| `risk_level` | tinyint | 否 | `0` |  | 风险等级：0=正常，9=封禁 |
| `tags` | json | 是 | - |  | 标签，如 ["high_roller","test"] |
| `bet_limit_override` | json | 是 | - |  | 注额上限覆盖，如 {"min":10000,"max":5000000} |
| `currency_code` | char(8) | 否 | `USD` |  | 币种编码（currencies.code） |
| `lifetime_bet_minor` | bigint | 否 | `0` |  | 累计下注额，minor units |
| `lifetime_win_minor` | bigint | 否 | `0` |  | 累计派彩额，minor units |
| `lifetime_rounds` | bigint unsigned | 否 | `0` |  | 累计局数 |
| `last_played_at` | datetime(3) | 是 | - |  | 最近游戏时间（UTC） |
| `status` | tinyint | 否 | `1` | IDX | 1=正常 0=暂停 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_last_played`（普通）：`merchant_id`, `last_played_at`
- `idx_merchant_risk`（普通）：`merchant_id`, `risk_level`
- `idx_merchant_vip`（普通）：`merchant_id`, `vip_level`, `status`
- `idx_status_last_played`（普通）：`status`, `last_played_at`
- `PRIMARY`（主键）：`id`
- `uk_merchant_user`（唯一）：`merchant_id`, `user_id`

### E. 会话、审计与运维

#### `game_sessions`

> 玩家会话登记

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `session_token_hash` | char(64) | 否 | - | UK | 会话令牌的 SHA-256，禁止存明文令牌 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `merchant_code` | varchar(64) | 否 | - |  | 商户编码 |
| `user_id` | bigint unsigned | 否 | - |  | 玩家 ID |
| `game_id` | bigint unsigned | 是 | - |  | 游戏 ID（games.id） |
| `game_code` | varchar(64) | 否 | - | IDX | 游戏编码 |
| `currency_code` | char(8) | 否 | `USD` |  | 币种编码（currencies.code） |
| `client_seed` | varchar(128) | 是 | - |  | 客户端种子 |
| `server_seed_hash` | varchar(128) | 是 | - |  | 服务端种子的 SHA-256（开局承诺） |
| `client_ip` | varchar(45) | 是 | - |  | 客户端 IP（IPv4 或 IPv6） |
| `user_agent` | varchar(512) | 是 | - |  | 客户端 User-Agent |
| `status` | varchar(16) | 否 | `active` |  | 状态：active/expired/revoked |
| `round_count` | int unsigned | 否 | `0` |  | 本次会话内的下注局数 |
| `expires_at` | datetime(3) | 否 | - | IDX | 过期时间（UTC），NULL=永久 |
| `last_active_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 最近活跃时间（UTC） |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` | IDX | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_created_status`（普通）：`created_at`, `status`
- `idx_expires_status`（普通）：`expires_at`, `status`
- `idx_game_status_expires`（普通）：`game_code`, `status`, `expires_at`
- `idx_merchant_user_active`（普通）：`merchant_id`, `user_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_session_token_hash`（唯一）：`session_token_hash`

#### `audit_logs`

> 后台操作审计日志

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `admin_user_id` | bigint unsigned | 是 | - | IDX | 后台用户 ID（admin_users.id） |
| `username` | varchar(64) | 否 | - |  | 用户名 |
| `role_name` | varchar(32) | 否 | - |  | 角色名 |
| `action` | varchar(64) | 否 | - | IDX | 操作动作，如 create_merchant/rotate_key/confirm_settlement |
| `resource_type` | varchar(64) | 否 | - | IDX | 资源类型：merchant/game/config/user/settlement |
| `resource_id` | varchar(128) | 否 | - |  | 资源 ID |
| `http_method` | varchar(8) | 是 | - |  | HTTP 方法 |
| `request_path` | varchar(256) | 是 | - |  | 请求路径 |
| `detail` | json | 是 | - |  | 请求快照或变更差异 |
| `client_ip` | varchar(45) | 是 | - |  | 客户端 IP |
| `status_code` | smallint | 是 | - |  | HTTP 状态码 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` | IDX | 创建时间（UTC） |

**索引**

- `idx_action_created`（普通）：`action`, `created_at`
- `idx_admin_user_created`（普通）：`admin_user_id`, `created_at`
- `idx_created`（普通）：`created_at`
- `idx_created_action`（普通）：`created_at`, `action`
- `idx_resource`（普通）：`resource_type`, `resource_id`
- `PRIMARY`（主键）：`id`

#### `game_maintenance_windows`

> 游戏维护窗口

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `game_id` | bigint unsigned | 是 | - | IDX | NULL=全部游戏 |
| `merchant_id` | bigint unsigned | 是 | - |  | NULL=平台级维护 |
| `title` | varchar(128) | 否 | - |  | 标题 |
| `reason` | varchar(512) | 是 | - |  | 原因 |
| `starts_at` | datetime(3) | 否 | - |  | 开始时间（UTC） |
| `ends_at` | datetime(3) | 否 | - |  | 结束时间（UTC） |
| `status` | varchar(16) | 否 | `scheduled` | IDX | 状态：scheduled/active/completed/cancelled |
| `created_by` | bigint unsigned | 是 | - |  | 创建人（admin_users.id） |
| `cancelled_by` | bigint unsigned | 是 | - |  | 取消人（admin_users.id） |
| `cancelled_at` | datetime(3) | 是 | - |  | 取消时间（UTC） |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_game_merchant_starts`（普通）：`game_id`, `merchant_id`, `starts_at`
- `idx_status_window`（普通）：`status`, `starts_at`, `ends_at`
- `PRIMARY`（主键）：`id`

#### `api_rate_limits`

> 商户 API 限流

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `limit_scope` | varchar(32) | 否 | `global` |  | 限流范围：global/bet/session/ip |
| `limit_type` | varchar(32) | 否 | `rps` |  | 限流类型：rps/daily_quota/concurrent |
| `limit_value` | int unsigned | 否 | - |  | 限流阈值 |
| `window_seconds` | int unsigned | 否 | `1` |  | 时间窗口（秒） |
| `burst` | int unsigned | 是 | - |  | 令牌桶突发容量 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_status`（普通）：`merchant_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_merchant_scope_type`（唯一）：`merchant_id`, `limit_scope`, `limit_type`

#### `merchant_webhooks`

> 商户出站 Webhook

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `event_type` | varchar(64) | 否 | - |  | 事件类型：settlement.confirmed/bigwin/risk.alert |
| `target_url` | varchar(512) | 否 | - |  | 回调目标地址 |
| `secret` | varchar(256) | 是 | - |  | HMAC 签名密钥 |
| `headers` | json | 是 | - |  | 附加 HTTP 请求头 |
| `retry_max` | tinyint | 否 | `5` |  | 最大重试次数 |
| `timeout_ms` | int | 否 | `5000` |  | 超时毫秒数 |
| `status` | tinyint | 否 | `1` | IDX | 1=启用 0=停用 |
| `last_triggered_at` | datetime(3) | 是 | - |  | 最近触发时间（UTC） |
| `last_status_code` | smallint | 是 | - |  | 最近一次回调的 HTTP 状态码 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_merchant_status`（普通）：`merchant_id`, `status`
- `idx_status_event`（普通）：`status`, `event_type`
- `PRIMARY`（主键）：`id`
- `uk_merchant_event`（唯一）：`merchant_id`, `event_type`

### F. 多语言（i18n）

#### `locales`

> 平台支持语言

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `code` | varchar(16) | 否 | - | PK | BCP 47 语言标签，如 en-US、zh-CN |
| `name` | varchar(64) | 否 | - |  | 英文显示名 |
| `native_name` | varchar(64) | 否 | - |  | 该语言自身的名称 |
| `direction` | char(3) | 否 | `ltr` |  | 书写方向：ltr 或 rtl |
| `sort_order` | int | 否 | `0` |  | 排序序号 |
| `is_default` | tinyint | 否 | `0` |  | 平台默认回退语言 |
| `status` | tinyint | 否 | `1` | IDX | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_status_sort`（普通）：`status`, `sort_order`
- `PRIMARY`（主键）：`code`

#### `i18n_bundles`

> i18n 词典命名空间

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `bundle_code` | varchar(64) | 否 | - | UK | 命名空间，如 ui.admin |
| `description` | varchar(256) | 是 | - |  | 说明 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `PRIMARY`（主键）：`id`
- `uk_bundle_code`（唯一）：`bundle_code`

#### `i18n_messages`

> i18n 词典键

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `bundle_id` | bigint unsigned | 否 | - | IDX | 词典命名空间 ID（i18n_bundles.id） |
| `message_key` | varchar(128) | 否 | - |  | 点分路径，如 nav.merchants |
| `default_value` | text | 否 | - |  | 缺少译文时的回退文案 |
| `description` | varchar(256) | 是 | - |  | 给译者的上下文说明 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_bundle_status`（普通）：`bundle_id`, `status`
- `PRIMARY`（主键）：`id`
- `uk_bundle_key`（唯一）：`bundle_id`, `message_key`

#### `i18n_message_translations`

> i18n 词典译文

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `message_id` | bigint unsigned | 否 | - | IDX | 词典键 ID（i18n_messages.id） |
| `locale_code` | varchar(16) | 否 | - | IDX | 语言标签（locales.code） |
| `translated_value` | text | 否 | - |  | 译文 |
| `status` | tinyint | 否 | `1` |  | 1=已发布 0=草稿 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_locale_status`（普通）：`locale_code`, `status`
- `PRIMARY`（主键）：`id`
- `uk_message_locale`（唯一）：`message_id`, `locale_code`

**外键**

- `locale_code` → `locales.code`
- `message_id` → `i18n_messages.id`

#### `i18n_entity_translations`

> 业务实体字段翻译

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `entity_type` | varchar(32) | 否 | - | IDX | 实体类型：game/category/merchant/role |
| `entity_id` | bigint unsigned | 否 | - |  | 业务实体 ID |
| `field_name` | varchar(32) | 否 | - |  | 字段名：name/description/title |
| `locale_code` | varchar(16) | 否 | - | IDX | 语言标签（locales.code） |
| `translated_value` | text | 否 | - |  | 译文 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `idx_entity_locale`（普通）：`entity_type`, `entity_id`, `locale_code`
- `idx_locale_type`（普通）：`locale_code`, `entity_type`, `status`
- `PRIMARY`（主键）：`id`
- `uk_entity_field_locale`（唯一）：`entity_type`, `entity_id`, `field_name`, `locale_code`

**外键**

- `locale_code` → `locales.code`

#### `merchant_locales`

> 商户启用语言

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `merchant_id` | bigint unsigned | 否 | - | IDX | 商户 ID（merchants.id） |
| `locale_code` | varchar(16) | 否 | - | IDX | 语言标签（locales.code） |
| `is_default` | tinyint | 否 | `0` |  | 商户默认语言 |
| `sort_order` | int | 否 | `0` |  | 排序序号 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `fk_merchant_locales_locale`（普通）：`locale_code`
- `idx_merchant_default`（普通）：`merchant_id`, `is_default`, `status`
- `PRIMARY`（主键）：`id`
- `uk_merchant_locale`（唯一）：`merchant_id`, `locale_code`

**外键**

- `locale_code` → `locales.code`

### G. 后台权限

#### `roles`

> 角色

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `name` | varchar(64) | 否 | - | UK | 名称 |
| `description` | varchar(256) | 是 | - |  | 说明 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `PRIMARY`（主键）：`id`
- `uk_role_name`（唯一）：`name`

#### `admin_users`

> 管理后台用户

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `username` | varchar(64) | 否 | - | UK | 用户名 |
| `password_hash` | varchar(256) | 否 | - |  | 密码哈希（bcrypt） |
| `role_id` | bigint unsigned | 否 | - | IDX | 角色 ID（roles.id） |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |
| `totp_secret` | varchar(64) | 是 | - |  | TOTP 密钥（加密存储） |
| `totp_enabled` | tinyint | 否 | `0` |  | 1=已启用二次验证 0=未启用 |
| `totp_recovery_hashes` | json | 是 | - |  | 一次性恢复码的 bcrypt 哈希 |

**索引**

- `idx_role_id`（普通）：`role_id`
- `PRIMARY`（主键）：`id`
- `uk_username`（唯一）：`username`

### H. 元数据与归档

#### `schema_migrations`

> 数据库迁移登记

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `version` | varchar(64) | 否 | - | UK | 迁移版本号 |
| `description` | varchar(256) | 是 | - |  | 说明 |
| `applied_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 应用时间（UTC） |

**索引**

- `PRIMARY`（主键）：`id`
- `uk_version`（唯一）：`version`

#### `schema_archive_policies`

> 表归档保留策略

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `id` | bigint unsigned | 否 | - | PK | 主键 |
| `table_name` | varchar(64) | 否 | - | UK | 表名 |
| `hot_retention_days` | int unsigned | 否 | `90` |  | MySQL 热层保留天数 |
| `cold_retention_days` | int unsigned | 是 | - |  | 清理前的冷存储保留天数（可选） |
| `archive_strategy` | varchar(32) | 否 | `delete` |  | 归档方式：delete/export_to_s3/partition_drop |
| `partition_column` | varchar(64) | 是 | - |  | 分区列，如按月分区的 created_at |
| `last_archived_at` | datetime(3) | 是 | - |  | 最近归档时间（UTC） |
| `notes` | varchar(512) | 是 | - |  | 备注 |
| `status` | tinyint | 否 | `1` |  | 状态：1=启用 0=停用 |
| `created_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 创建时间（UTC） |
| `updated_at` | datetime(3) | 否 | `CURRENT_TIMESTAMP(3)` |  | 更新时间（UTC） |

**索引**

- `PRIMARY`（主键）：`id`
- `uk_table_name`（唯一）：`table_name`

### I. 视图

#### `v_merchant_game_lobby`

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `merchant_id` | bigint unsigned | 否 | `0` |  | 主键 |
| `merchant_code` | varchar(64) | 否 | - |  | 商户唯一编码 |
| `merchant_name` | varchar(128) | 否 | - |  | 商户名称 |
| `merchant_game_id` | bigint unsigned | 否 | `0` |  | 主键 |
| `lobby_sort` | int | 否 | `0` |  | 商户大厅排序 |
| `merchant_game_status` | tinyint | 否 | `1` |  | 1=开通 0=关闭 |
| `merchant_min_bet_minor` | bigint | 是 | - |  | 覆盖最小注，NULL=用游戏默认 |
| `merchant_max_bet_minor` | bigint | 是 | - |  | 覆盖最大注 |
| `merchant_rtp_tier_code` | varchar(32) | 是 | - |  | 覆盖默认 RTP 档位 |
| `game_id` | bigint unsigned | 否 | `0` |  | 主键 |
| `game_code` | varchar(64) | 否 | - |  | 全局唯一游戏标识 |
| `game_name` | varchar(128) | 否 | - |  | 名称 |
| `game_type` | varchar(32) | 否 | - |  | 游戏类型：fishing/slot/crash/table |
| `default_rtp_ppm` | bigint | 否 | `960000` |  | 默认 RTP，96%=960000 |
| `game_min_bet_minor` | bigint | 否 | `10000` |  | 默认最小注 minor units |
| `game_max_bet_minor` | bigint | 否 | `10000000` |  | 默认最大注 minor units |
| `client_version` | varchar(32) | 是 | - |  | 当前客户端版本 |
| `thumbnail_url` | varchar(512) | 是 | - |  | 缩略图地址 |
| `game_status` | tinyint | 否 | `1` |  | 1=上架 0=下架 2=维护 |
| `category_code` | varchar(32) | 是 | - |  | 分类编码：fishing/slot/crash/table |
| `category_name` | varchar(64) | 是 | - |  | 名称 |
| `effective_min_bet_minor` | bigint | 否 | `0` |  | 视图计算列 |
| `effective_max_bet_minor` | bigint | 否 | `0` |  | 视图计算列 |
| `tier_rtp_ppm` | bigint | 是 | - |  | 目标 RTP ppm |

#### `v_merchant_game_lobby_i18n`

| 列 | 类型 | 空 | 默认 | 键 | 说明 |
|---|---|:--:|---|---|---|
| `merchant_id` | bigint unsigned | 否 | `0` |  | 主键 |
| `merchant_code` | varchar(64) | 否 | - |  | 商户唯一编码 |
| `locale_code` | varchar(16) | 否 | - |  | 语言标签（locales.code） |
| `game_id` | bigint unsigned | 否 | `0` |  | 主键 |
| `game_code` | varchar(64) | 否 | - |  | 全局唯一游戏标识 |
| `game_name` | mediumtext | 否 | - |  | 视图计算列 |
| `category_name` | mediumtext | 是 | - |  | 视图计算列 |
| `game_type` | varchar(32) | 否 | - |  | 游戏类型：fishing/slot/crash/table |
| `category_code` | varchar(32) | 是 | - |  | 分类编码：fishing/slot/crash/table |
| `lobby_sort` | int | 否 | `0` |  | 商户大厅排序 |
| `effective_min_bet_minor` | bigint | 否 | `0` |  | 视图计算列 |
| `effective_max_bet_minor` | bigint | 否 | `0` |  | 视图计算列 |
| `client_version` | varchar(32) | 是 | - |  | 当前客户端版本 |
| `thumbnail_url` | varchar(512) | 是 | - |  | 缩略图地址 |
| `tier_rtp_ppm` | bigint | 是 | - |  | 目标 RTP ppm |

## 五、表关系

除 i18n 相关表外，数据库层**没有物理外键**，关联全部由代码维护。以下 4 个是仅有的物理外键：

| 表 | 列 | 引用 |
|---|---|---|
| `i18n_entity_translations` | `locale_code` | `locales.code` |
| `i18n_message_translations` | `locale_code` | `locales.code` |
| `i18n_message_translations` | `message_id` | `i18n_messages.id` |
| `merchant_locales` | `locale_code` | `locales.code` |

逻辑关系：

```
merchants ─┬─< merchant_games >── games ─┬─< game_rtp_tiers
           │                              └── game_categories
           ├─< merchant_wallet_configs
           ├─< merchant_currencies >── currencies
           ├─< merchant_locales >───── locales
           ├─< commission_rules
           ├─< api_rate_limits
           ├─< merchant_webhooks
           ├─< game_configs           唯一键 (merchant_id, game_code, config_key)
           ├─< settlement_periods ─< merchant_settlement_lines
           ├─< daily_settlements ──> settlement_periods (settlement_period_id)
           └─< player_merchant_profiles

games ─┬─< game_client_versions
       └─< merchant_game_versions >── merchants

同一局（round_id）在三张表留痕：
  game_round_replay.round_id     无唯一键
  pending_transactions.round_id  uk_round_id
  wallet_pending_ops.round_id    uk_round_id + op_type

i18n：
  locales ─┬─< i18n_message_translations >── i18n_messages >── i18n_bundles
           ├─< i18n_entity_translations  业务实体字段翻译（game / category 名称）
           └─< merchant_locales >── merchants
```

## 六、视图说明

- **`v_merchant_game_lobby`**：商户游戏大厅。聚合 merchants + merchant_games + games + game_categories + game_rtp_tiers，用 COALESCE(mg.min_bet_minor, g.min_bet_minor) 计算生效注额（商户覆盖优先于游戏默认），tier_rtp_ppm 按 mg.rtp_tier_code 取档位 RTP。
- **`v_merchant_game_lobby_i18n`**：在大厅视图之上，按 merchant_locales 关联 i18n_entity_translations，输出多语言游戏名与分类名（COALESCE(译文, 原名)）。

> ⚠️ 两个视图目前**无任何 Go 代码引用**。RGS 的注额校验走 game_configs 里的 bet_limits JSON，而不是视图里的 effective_min_bet_minor。详见第九节。

## 七、迁移历史

登记在 `schema_migrations` 的迁移：

| version | 说明 |
|---|---|
| `11-platform-games` | Platform game catalog, merchant bindings, wallet configs |
| `12-operations-audit` | Game sessions, admin audit logs, maintenance windows |
| `13-settlement-billing` | Settlement periods and merchant settlement lines |
| `14-player-profiles` | Player merchant profiles with VIP and limits |
| `15-api-governance` | API rate limits and merchant webhooks |
| `16-game-versions` | Game client versions and merchant upgrade policy |
| `17-index-optimize` | Index optimization batch 1 |
| `18-index-optimize-2` | Index optimization batch 2 |
| `19-normalize-merchant-refs` | Add merchant_id to legacy merchant_code tables |
| `20-audit-archive-policy` | Archive retention policy for audit_logs and game_sessions |
| `21-merchant-lobby-view` | v_merchant_game_lobby view for game launch API |
| `22-status-checks` | CHECK constraints on status columns |
| `23-seed-extra-games` | Seed slot/crash placeholder games and demo merchant bindings |
| `24-i18n-dictionary` | Language locales, i18n message dictionary, entity translations |
| `25-i18n-admin-extra` | Additional admin panel i18n labels |
| `26-i18n-utf8-repair` | Repair i18n Chinese text using UTF-8 hex literals |
| `27-i18n-platform-games` | Admin i18n for platform games and merchant lobby |
| `28-i18n-audit-dashboard` | Admin i18n for audit logs and dashboard |
| `29-repair-column-comments` | Restore Chinese column comments damaged by wrong charset |
| `30-translate-comments` | Translate English table/column comments into Simplified Chinese |
| `32-outbox` | Transactional outbox (event_outbox) + consumer dedup (processed_events) |
| `33-merchant-scoped-keys` | Scope unique keys by merchant_id on reconciliation tables |
| `34-round-id-index` | Add single-column round_id indexes for merchant-unscoped read paths |
| `35-ledger` | Land the ledger: transaction_types + game_transactions + player_accounts |
| `36-ledger-idempotency-by-status` | Include terminal status in ledger idempotency key so compensation entries can coexist |

> 注意：03 / 07 / 09 / 10 四个迁移脚本**未登记** `schema_migrations`（只有 11–28 登记了）。排查历史请一并查看 `docker/mysql/init/` 下的原始文件。

## 八、模型覆盖情况

`internal/model/` 只为 **12 张表**提供了模型，其余 **29 张基表无模型**：

| 有模型 | 无模型 |
|---|---|
| `admin_users` | `merchant_game_versions` |
| `audit_logs` | `game_rtp_tiers` |
| `daily_settlements` | `game_categories` |
| `game_configs` | `processed_events` |
| `game_round_replay` | `i18n_messages` |
| `merchants` | `api_rate_limits` |
| `pending_transactions` | `player_accounts` |
| `risk_alerts` | `locales` |
| `risk_blacklist` | `i18n_message_translations` |
| `roles` | `transaction_types` |
| `settlement_periods` | `i18n_entity_translations` |
| `wallet_pending_ops` | `merchant_currencies` |
|  | `merchant_locales` |
|  | `commission_rules` |
|  | `merchant_settlement_lines` |
|  | `game_sessions` |
|  | `event_outbox` |
|  | `player_merchant_profiles` |
|  | `game_client_versions` |
|  | `game_maintenance_windows` |
|  | `schema_migrations` |
|  | `merchant_wallet_configs` |
|  | `games` |
|  | `currencies` |
|  | `merchant_webhooks` |
|  | `game_transactions` |
|  | `merchant_games` |
|  | `schema_archive_policies` |
|  | `i18n_bundles` |

> 无模型不等于无用：games / game_rtp_tiers / merchant_games / game_categories / i18n 系列都在 `platformGamesModel.go`、`i18nModel.go` 里用**手写 SQL** 访问，只是没有 goctl 生成的模型文件。真正零引用的表见第九节。

## 九、重复定义与已知问题

### 9.1 真正的重复定义（同一语义存于多处）

| # | 语义 | 冲突位置 | 实际生效方 |
|---|---|---|---|
| 1 | 下注限额 | `game_configs.bet_limits`（JSON） vs `merchant_games.min_bet_minor/max_bet_minor` vs `games.min_bet_minor/max_bet_minor` | **game_configs**：RGS 读它（betLogic.go:96 → gameconfig.Loader），另两者只被 Admin 写入、无人读 |
| 2 | RTP 档位 | `games.default_rtp_ppm` / `game_rtp_tiers.target_rtp_ppm` / `merchant_games.rtp_tier_code` / `game_configs.rtp_tier` | **都不生效**：pkg/prng 的分支硬编码，rtpTier 只作标签回填 |
| 3 | 客户端版本 | `games.client_version`（varchar） vs `game_client_versions`（整表） | **games.client_version**；后者无代码引用 |
| 4 | 货币 | `currencies` vs `merchant_currencies` | **都不生效**：两表均无代码引用 |
| 5 | 钱包配置 | merchants 表的密钥字段 vs `merchant_wallet_configs` | **YAML 配置**：钱包参数读 services/*/etc/*.yaml，表无引用 |
| 6 | 商户分润 | `commission_rules` vs `settlement_periods.commission_minor` | **settlement_periods**：佣金在结算周期里直接列存 |
| 7 | 结算金额 | `daily_settlements.total_bet/total_win` vs `settlement_periods.total_bet_minor/total_win_minor` | 日结与周期结算是两级汇总（保留），但金额列命名不一致（前者无 minor 后缀） |

### 9.2 零代码引用的表

以下对象在 Go 代码中**完全没有任何引用**（既无模型，也无手写 SQL）：

- `api_rate_limits`
- `commission_rules`
- `currencies`
- `game_client_versions`
- `game_maintenance_windows`
- `game_sessions`
- `i18n_entity_translations`
- `merchant_currencies`
- `merchant_game_versions`
- `merchant_locales`
- `merchant_settlement_lines`
- `merchant_wallet_configs`
- `merchant_webhooks`
- `player_merchant_profiles`
- `schema_archive_policies`
- `schema_migrations`
- `v_merchant_game_lobby`
- `v_merchant_game_lobby_i18n`

> schema_migrations 由迁移脚本自身读写，两个视图供查询/报表使用，这两类属于「非代码路径」，不算冗余。其余属于**设计完成但功能未实现**。

### 9.3 模型与表结构的一致性

模型分两类，**只有第一类会真正出错**：

- **goctl 生成的** `*_gen.go`：用 `builder.RawFieldNames(&Struct{})` 动态拼列名，结构体缺字段会让生成的 SELECT/INSERT/UPDATE **直接少列**，编译期无法发现。
- **手写模型**：用显式列名（如 `rolesModel.go` 的 select id, name, description），结构体是按需投影，少字段只是不查那一列，**不是 bug**。

#### 已修复（goctl 生成模型缺列 → 生成的 SQL 缺列）

| 表 | 模型文件 | 原缺失列 | 影响 |
|---|---|---|---|
| `merchants` | merchantsModel_gen.go | private_key_prev, private_key_prev_expires_at, allowed_ips | SELECT 查不到密钥轮换与 IP 白名单；INSERT/UPDATE 语句缺列（列数与占位符数不匹配） |
| `admin_users` | adminUsersModel_gen.go | totp_secret, totp_enabled, totp_recovery_hashes | INSERT 只有 4 个占位符却要写 7 列，**建管理员账号必失败**；登录取不到 2FA 字段 |

修复内容：补齐结构体字段，并同步修正 goctl 硬编码的占位符串与参数列表。已用真实数据库执行 CRUD 验证。

#### 手写模型的"缺失列"（按需投影，无需修复）

| 表 | 模型文件 | 未纳入的列 | 说明 |
|---|---|---|---|
| `roles` | rolesModel.go | created_at, updated_at | 显式 select 三列，用不到时间戳 |
| `admin_users` | adminAuthModel.go | created_at, updated_at | 登录鉴权查询，用不到 |
| `wallet_pending_ops` | walletPendingOpsModel.go | merchant_id | 迁移 19 新增列；当前按 merchant_code 工作，未使用该列 |
| `risk_alerts` | riskAlertsModel.go | merchant_id | 同上 |
| `pending_transactions` | pendingTransactionsModel.go | merchant_id | 同上 |
| `game_round_replay` | gameRoundReplayModel.go | merchant_id | 同上 |
| `daily_settlements` | dailySettlementsModel.go | settlement_period_id | 迁移 13 新增列；周期汇总由 settlementPeriodsModel 独立处理 |

**根因**：迁移 13 / 19 等 ALTER 加列后，没有重新运行 goctl。生成的模型因此停留在旧表结构上。

#### 防止再次漂移

新增了 `internal/model/schema_consistency_test.go`，直接拿真实库比对模型声明的列，并真跑一遍生成的 CRUD SQL：

```powershell
# 需要 fastgame-mysql 在线（未设置 MYSQL_DSN 时测试自动 skip）
go test ./internal/model/ -run TestSchema -v
```

**今后任何 ALTER 之后都应跑一次这个测试。**

### 9.4 库内注释损坏（已修复）

线上库里曾有 **29 处注释**被错误 charset 毁掉（8 张表的 28 个列注释 + 1 个表注释），两种形态（均用 HEX 确认）：

| 形态 | 例子 | HEX 证据 | 可否直接还原 |
|---|---|---|---|
| 中文被替换成 ASCII 问号（含单个汉字被吃成一个 ?） | `games.game_code` = "????????"、`game_rtp_tiers.tier_code` 末尾的 "等" 变成 "?" | `3F3F3F3F...` | 回不来，但可从迁移文件取回原文 |
| 双重编码乱码（UTF-8 被当 Latin-1 再编码） | `pending_transactions` 表注释、`pending_transactions.trace_id` | `C3A5C2AD...` | 回不来，但可从迁移文件取回原文 |

成因：早期迁移脚本由未指定 charset 的客户端执行，中文在写入时就已损坏。

**修复方式**：以 `docker/mysql/init/*.sql` 为权威来源重建注释定义。

| 步骤 | 命令 / 产物 |
|---|---|
| 1. 提取权威注释 | `.\scripts\gen_comment_manifest.ps1` → `docker/mysql/migrations/comment_manifest.json` |
| 2. 生成修复迁移 | `.\scripts\gen_comment_repair.ps1` |
| 3. 应用 | `docker/mysql/migrations/29-repair-column-comments-migration.sql`（回滚脚本同目录） |

修复要点：列定义由 `information_schema` 的 `COLUMN_TYPE/IS_NULLABLE/COLUMN_DEFAULT/EXTRA` 原样重建，**只替换 COMMENT**，因此类型、默认值、`auto_increment`、`on update CURRENT_TIMESTAMP` 均无漂移（已比对修复前后 393 列定义，完全一致）。

**修复后状态：36 个表注释 + 393 个列注释全部为简体中文，0 处不一致、0 处问号、0 处缺失。**

### 9.5 注释全量中文化（已应用）

注释先修复（9.4），再统一中文化：原先有 27 个表注释与 83 个列注释是英文，另有 264 个列完全没有注释。

| 步骤 | 命令 / 产物 |
|---|---|
| 1. 编写译法 | `docker/mysql/migrations/translations_comments.json`（精确译法）、`translations_common_columns.json`（通用列名词典） |
| 2. 生成迁移 | `.\scripts\gen_comment_translate.ps1` |
| 3. 应用 | `docker/mysql/migrations/30-translate-comments-migration.sql`（回滚脚本同目录） |
| 4. 同步 Go 模型 | `.\scripts\annotate_models_zh.ps1` — 用库中注释为模型字段补中文注释 |

翻译约定：枚举值与专有名词保留英文（如 

**当前状态：**

| 对象 | 数量 | 中文注释 | 无注释 |
|---|---:|---:|---:|
| 基表 | 36 | 36 | 0 |
| 基表列 | 393 | 393 | 0 |
| 视图计算列 | 16 | 标注为「视图计算列」 | — |

Go 模型字段注释由 `scripts/annotate_models_zh.ps1` 自动同步自库中注释，两者不会漂移。

### 9.6 其他

- 旧文档记载的表数与实际不符（曾写 35 张，实际 36 张基表 + 2 视图），以本文档为准。

## 十、维护命令

```powershell
# 应用单个迁移
.\scripts\apply-migration.ps1 24-i18n-dictionary
# 应用全部
.\scripts\apply-all-migrations.ps1
# 重新生成本文档
powershell -ExecutionPolicy Bypass -File .\scripts\gen_database_doc.ps1
# 校验模型与实际表结构是否一致（ALTER 之后必跑）
go test ./internal/model/ -run TestSchema -v
# 注释维护：提取权威注释 -> 生成修复/翻译迁移 -> 同步 Go 模型
powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_manifest.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_repair.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\gen_comment_translate.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\annotate_models_zh.ps1 -DryRun
# 表数量核对
docker exec fastgame-mysql mysql --default-character-set=utf8mb4 -ufastgame -pfastgame_pass fastgame -e "SELECT TABLE_TYPE, COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='fastgame' GROUP BY TABLE_TYPE;"
```

---

*本文件由脚本生成。如需修改结构说明，请改 `scripts/gen_database_doc.ps1` 后重新生成。*
