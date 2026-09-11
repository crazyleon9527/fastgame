# FastGame MySQL 数据库设计

> 多商户 · 自研多游戏 · MySQL 8.0 · utf8mb4 · UTC  
> **当前：35 张基表 + 2 个视图（v_merchant_game_lobby / v_merchant_game_lobby_i18n）**

## 设计原则

1. **平台层 vs 商户层**：游戏目录、RTP 档位等平台统一维护；商户通过 `merchant_games` 开通/定制。
2. **注单明细在 ClickHouse**：MySQL 仅存配置、会话、对账、审计；海量注单不入 MySQL。
3. **金额用 minor units (BIGINT)**：与 `pkg/money` 一致，4 位小数 ×10000。
4. **RTP 用 PPM**：96% = `960000`（parts per million），避免浮点。
5. **迁移可重复执行**：新脚本使用 `CREATE TABLE IF NOT EXISTS` / 条件 `ALTER`。

## 全表清单（31）

| # | 表 | 模块 |
|---|-----|------|
| 1 | merchants | 商户 |
| 2 | merchant_games | 商户 |
| 3 | merchant_wallet_configs | 商户 |
| 4 | merchant_currencies | 商户 |
| 5 | commission_rules | 商户 |
| 6 | api_rate_limits | 商户/API |
| 7 | merchant_webhooks | 商户/API |
| 8 | games | 游戏平台 |
| 9 | game_categories | 游戏平台 |
| 10 | game_rtp_tiers | 游戏平台 |
| 11 | game_configs | 游戏平台 |
| 12 | game_round_replay | 游戏平台 |
| 13 | game_client_versions | 游戏平台 |
| 14 | merchant_game_versions | 游戏平台 |
| 15 | currencies | 主数据 |
| 16 | daily_settlements | 结算 |
| 17 | settlement_periods | 结算 |
| 18 | merchant_settlement_lines | 结算 |
| 19 | wallet_pending_ops | 钱包补偿 |
| 20 | pending_transactions | 钱包补偿 |
| 21 | risk_blacklist | 风控 |
| 22 | risk_alerts | 风控 |
| 23 | game_sessions | 运营 |
| 24 | audit_logs | 运营 |
| 25 | game_maintenance_windows | 运营 |
| 26 | player_merchant_profiles | 玩家 |
| 27 | roles | 管理后台 |
| 28 | admin_users | 管理后台 |
| 29 | schema_archive_policies | 元数据 / 归档策略 |
| 30 | schema_migrations | 元数据 |
| 31 | locales | 多语言 |
| 32 | i18n_bundles | 多语言 |
| 33 | i18n_messages | 多语言 |
| 34 | i18n_message_translations | 多语言 |
| 35 | i18n_entity_translations | 多语言 |
| 36 | merchant_locales | 多语言 |

## 语言词典（24）

```
locales ──< merchant_locales >── merchants
   │
   ├──< i18n_message_translations >── i18n_messages >── i18n_bundles
   └──< i18n_entity_translations (game/category/merchant fields)
```

| 表 | 用途 |
|----|------|
| locales | 平台支持语言（BCP 47：en-US / zh-CN / zh-TW …） |
| i18n_bundles | 词典命名空间（ui.admin / ui.client / api.errors） |
| i18n_messages | 词典键 + 默认 fallback 文案 |
| i18n_message_translations | 各语言翻译值 |
| i18n_entity_translations | 业务实体字段翻译（游戏名、分类名等） |
| merchant_locales | 商户启用语言及默认语言 |

**查询示例：**

```sql
-- 管理后台 ui.admin 包，取 zh-CN 词典（缺省回退 default_value）
SELECT m.message_key,
       COALESCE(t.translated_value, m.default_value) AS label
FROM i18n_messages m
JOIN i18n_bundles b ON b.id = m.bundle_id AND b.bundle_code = 'ui.admin'
LEFT JOIN i18n_message_translations t ON t.message_id = m.id AND t.locale_code = 'zh-CN'
WHERE m.status = 1;

-- 商户大厅多语言列表
SELECT * FROM v_merchant_game_lobby_i18n
WHERE merchant_code = 'm001' AND locale_code = 'zh-CN';
```

## 迁移版本（01–24）

| 版本 | 内容 |
|------|------|
| 01–10 | 核心商户、RBAC、风控、钱包补偿、回放 |
| 11 | 平台游戏目录、商户开通、钱包配置、分成 |
| 12 | 会话、审计、维护窗口 |
| 13 | 结算周期、明细行 |
| 14 | 玩家档案（VIP/标签/限额） |
| 15 | API 限流、Webhook |
| 16 | 客户端版本、商户升级策略 |
| 17–18 | 索引优化批次 1–2 |
| 19 | merchant_id 回填（risk_alerts / wallet_pending_ops 等） |
| 20 | schema_archive_policies 归档策略表 |
| 21 | v_merchant_game_lobby 视图 |
| 22 | status CHECK 约束 |
| 23 | slot-demo / crash-demo 种子游戏 |
| 24 | 语言词典 + 多语言大厅视图 |

## 维护阶段（循环任务）

功能表与首轮优化已完成。`db-schema-loop.ps1` 每 10 分钟执行 `recurring_checks`：

- EXPLAIN 慢查询、索引使用率
- 归档策略 dry-run
- merchant_id 回填完整性

## 常用命令

```powershell
# 一次性应用 14–18
.\scripts\apply-all-migrations.ps1

# 单条迁移
.\scripts\apply-migration.ps1 16-game-versions

# 优化循环（10 分钟）
.\scripts\db-schema-loop.ps1

# 查看表数量
docker exec fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame -N -e "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='fastgame';"
```
