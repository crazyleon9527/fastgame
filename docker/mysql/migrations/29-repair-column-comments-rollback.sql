-- 29-repair-column-comments-rollback.sql
-- 回滚：把表/列注释恢复为修复前的值（HEX 确认过的原始损坏内容）。
-- 由 scripts/gen_comment_repair.ps1 生成。
USE fastgame;

SET NAMES utf8mb4;

-- 表注释
ALTER TABLE `api_rate_limits` COMMENT = '商户 API 限流';
ALTER TABLE `audit_logs` COMMENT = '后台操作审计日志';
ALTER TABLE `commission_rules` COMMENT = '商户分润规则';
ALTER TABLE `currencies` COMMENT = '币种主数据';
ALTER TABLE `game_categories` COMMENT = '游戏分类';
ALTER TABLE `game_client_versions` COMMENT = '游戏客户端版本发布';
ALTER TABLE `game_maintenance_windows` COMMENT = '游戏维护窗口';
ALTER TABLE `game_round_replay` COMMENT = '确定性回放输入';
ALTER TABLE `game_rtp_tiers` COMMENT = '游戏 RTP 档位';
ALTER TABLE `game_sessions` COMMENT = '玩家会话登记';
ALTER TABLE `games` COMMENT = '平台游戏目录';
ALTER TABLE `i18n_bundles` COMMENT = 'i18n 词典命名空间';
ALTER TABLE `i18n_entity_translations` COMMENT = '业务实体字段翻译';
ALTER TABLE `i18n_message_translations` COMMENT = 'i18n 词典译文';
ALTER TABLE `i18n_messages` COMMENT = 'i18n 词典键';
ALTER TABLE `locales` COMMENT = '平台支持语言';
ALTER TABLE `merchant_currencies` COMMENT = '商户币种';
ALTER TABLE `merchant_game_versions` COMMENT = '商户游戏版本策略';
ALTER TABLE `merchant_games` COMMENT = '商户游戏开通';
ALTER TABLE `merchant_locales` COMMENT = '商户启用语言';
ALTER TABLE `merchant_settlement_lines` COMMENT = '结算周期明细行';
ALTER TABLE `merchant_wallet_configs` COMMENT = '商户钱包配置';
ALTER TABLE `merchant_webhooks` COMMENT = '商户出站 Webhook';
ALTER TABLE `player_merchant_profiles` COMMENT = '商户下玩家档案';
ALTER TABLE `schema_archive_policies` COMMENT = '表归档保留策略';
ALTER TABLE `schema_migrations` COMMENT = '数据库迁移登记';
ALTER TABLE `settlement_periods` COMMENT = '商户结算周期账单';

-- api_rate_limits 列注释
ALTER TABLE `api_rate_limits` MODIFY COLUMN `burst` int unsigned NULL COMMENT '令牌桶突发容量';
ALTER TABLE `api_rate_limits` MODIFY COLUMN `limit_scope` varchar(32) NOT NULL DEFAULT 'global' COMMENT '限流范围：global/bet/session/ip';
ALTER TABLE `api_rate_limits` MODIFY COLUMN `limit_type` varchar(32) NOT NULL DEFAULT 'rps' COMMENT '限流类型：rps/daily_quota/concurrent';

-- audit_logs 列注释
ALTER TABLE `audit_logs` MODIFY COLUMN `action` varchar(64) NOT NULL COMMENT '操作动作，如 create_merchant/rotate_key/confirm_settlement';
ALTER TABLE `audit_logs` MODIFY COLUMN `detail` json NULL COMMENT '请求快照或变更差异';
ALTER TABLE `audit_logs` MODIFY COLUMN `resource_type` varchar(64) NOT NULL COMMENT '资源类型：merchant/game/config/user/settlement';

-- commission_rules 列注释
ALTER TABLE `commission_rules` MODIFY COLUMN `rule_type` varchar(32) NOT NULL DEFAULT 'ggr_share' COMMENT '分润方式：ggr_share/fixed_fee';

-- game_categories 列注释
ALTER TABLE `game_categories` MODIFY COLUMN `code` varchar(32) NOT NULL COMMENT '分类编码：fishing/slot/crash/table';

-- game_client_versions 列注释
ALTER TABLE `game_client_versions` MODIFY COLUMN `bundle_hash` char(64) NULL COMMENT '包体 SHA-256';
ALTER TABLE `game_client_versions` MODIFY COLUMN `bundle_url` varchar(512) NULL COMMENT 'CDN 路径或产物地址';
ALTER TABLE `game_client_versions` MODIFY COLUMN `min_engine_version` varchar(32) NULL COMMENT '要求的引擎版本（Cocos）';
ALTER TABLE `game_client_versions` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'draft' COMMENT '状态：draft/published/deprecated';
ALTER TABLE `game_client_versions` MODIFY COLUMN `version` varchar(32) NOT NULL COMMENT '语义化版本，如 1.2.0';

-- game_maintenance_windows 列注释
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `created_by` bigint unsigned NULL COMMENT '创建人（admin_users.id）';
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `game_id` bigint unsigned NULL COMMENT 'NULL=全部游戏';
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `merchant_id` bigint unsigned NULL COMMENT 'NULL=平台级维护';
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'scheduled' COMMENT '状态：scheduled/active/completed/cancelled';

-- game_round_replay 列注释
ALTER TABLE `game_round_replay` MODIFY COLUMN `round_id` varchar(64) NOT NULL COMMENT '局 ID，同时作为 nonce';

-- game_sessions 列注释
ALTER TABLE `game_sessions` MODIFY COLUMN `client_ip` varchar(45) NULL COMMENT '客户端 IP（IPv4 或 IPv6）';
ALTER TABLE `game_sessions` MODIFY COLUMN `round_count` int unsigned NOT NULL DEFAULT '0' COMMENT '本次会话内的下注局数';
ALTER TABLE `game_sessions` MODIFY COLUMN `session_token_hash` char(64) NOT NULL COMMENT '会话令牌的 SHA-256，禁止存明文令牌';
ALTER TABLE `game_sessions` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'active' COMMENT '状态：active/expired/revoked';

-- games 列注释
ALTER TABLE `games` MODIFY COLUMN `game_type` varchar(32) NOT NULL COMMENT '游戏类型：fishing/slot/crash/table';
ALTER TABLE `games` MODIFY COLUMN `volatility` varchar(16) NULL COMMENT '波动性：low/medium/high';

-- i18n_bundles 列注释
ALTER TABLE `i18n_bundles` MODIFY COLUMN `bundle_code` varchar(64) NOT NULL COMMENT '命名空间，如 ui.admin';

-- i18n_entity_translations 列注释
ALTER TABLE `i18n_entity_translations` MODIFY COLUMN `entity_type` varchar(32) NOT NULL COMMENT '实体类型：game/category/merchant/role';
ALTER TABLE `i18n_entity_translations` MODIFY COLUMN `field_name` varchar(32) NOT NULL COMMENT '字段名：name/description/title';

-- i18n_message_translations 列注释
ALTER TABLE `i18n_message_translations` MODIFY COLUMN `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=已发布 0=草稿';

-- i18n_messages 列注释
ALTER TABLE `i18n_messages` MODIFY COLUMN `default_value` text NOT NULL COMMENT '缺少译文时的回退文案';
ALTER TABLE `i18n_messages` MODIFY COLUMN `description` varchar(256) NULL COMMENT '给译者的上下文说明';
ALTER TABLE `i18n_messages` MODIFY COLUMN `message_key` varchar(128) NOT NULL COMMENT '点分路径，如 nav.merchants';

-- locales 列注释
ALTER TABLE `locales` MODIFY COLUMN `code` varchar(16) NOT NULL COMMENT 'BCP 47 语言标签，如 en-US、zh-CN';
ALTER TABLE `locales` MODIFY COLUMN `direction` char(3) NOT NULL DEFAULT 'ltr' COMMENT '书写方向：ltr 或 rtl';
ALTER TABLE `locales` MODIFY COLUMN `is_default` tinyint NOT NULL DEFAULT '0' COMMENT '平台默认回退语言';
ALTER TABLE `locales` MODIFY COLUMN `name` varchar(64) NOT NULL COMMENT '英文显示名';
ALTER TABLE `locales` MODIFY COLUMN `native_name` varchar(64) NOT NULL COMMENT '该语言自身的名称';

-- merchant_game_versions 列注释
ALTER TABLE `merchant_game_versions` MODIFY COLUMN `min_version_id` bigint unsigned NULL COMMENT '低于此版本强制升级';
ALTER TABLE `merchant_game_versions` MODIFY COLUMN `rollout_percent` tinyint NOT NULL DEFAULT '100' COMMENT '灰度发布比例 0-100';

-- merchant_locales 列注释
ALTER TABLE `merchant_locales` MODIFY COLUMN `is_default` tinyint NOT NULL DEFAULT '0' COMMENT '商户默认语言';

-- merchant_settlement_lines 列注释
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `amount_minor` bigint NOT NULL DEFAULT '0' COMMENT '带符号的明细金额';
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `commission_rate_ppm` bigint NULL COMMENT '结算时的费率快照';
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `daily_settlement_id` bigint unsigned NULL COMMENT '关联 daily_settlements.id';
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `ref_date` date NULL COMMENT 'daily_aggregate 类型对应的日期';

-- merchant_wallet_configs 列注释
ALTER TABLE `merchant_wallet_configs` MODIFY COLUMN `wallet_type` varchar(32) NOT NULL DEFAULT 'seamless' COMMENT '钱包类型：seamless/transfer/mock';

-- merchant_webhooks 列注释
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `event_type` varchar(64) NOT NULL COMMENT '事件类型：settlement.confirmed/bigwin/risk.alert';
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `headers` json NULL COMMENT '附加 HTTP 请求头';
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `secret` varchar(256) NULL COMMENT 'HMAC 签名密钥';
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=启用 0=停用';

-- pending_transactions 列注释
ALTER TABLE `pending_transactions` MODIFY COLUMN `expected_action` varchar(16) NOT NULL COMMENT '期望补偿动作：settle_win/rollback_bet/none';
ALTER TABLE `pending_transactions` MODIFY COLUMN `phase` varchar(32) NOT NULL COMMENT '阶段：bet_debited/win_pending/settled/orphan';
ALTER TABLE `pending_transactions` MODIFY COLUMN `round_id` varchar(64) NOT NULL COMMENT '局 ID，同时作为 nonce';
ALTER TABLE `pending_transactions` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT '状态：pending/done/failed';

-- player_merchant_profiles 列注释
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `bet_limit_override` json NULL COMMENT '注额上限覆盖，如 {"min":10000,"max":5000000}';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `external_user_ref` varchar(128) NULL COMMENT '商户侧的玩家字符串 ID（可选）';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `risk_level` tinyint NOT NULL DEFAULT '0' COMMENT '风险等级：0=正常，9=封禁';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=正常 0=暂停';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `tags` json NULL COMMENT '标签，如 ["high_roller","test"]';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `user_id` bigint unsigned NOT NULL COMMENT '商户体系内的玩家 ID';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `vip_level` tinyint NOT NULL DEFAULT '0' COMMENT 'VIP 等级：0=普通，越大越高';

-- risk_alerts 列注释
ALTER TABLE `risk_alerts` MODIFY COLUMN `scope_type` varchar(16) NOT NULL COMMENT '作用域：user 或 game';

-- risk_blacklist 列注释
ALTER TABLE `risk_blacklist` MODIFY COLUMN `list_type` varchar(32) NOT NULL COMMENT '名单类型：ip / user_id / merchant';

-- schema_archive_policies 列注释
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `archive_strategy` varchar(32) NOT NULL DEFAULT 'delete' COMMENT '归档方式：delete/export_to_s3/partition_drop';
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `cold_retention_days` int unsigned NULL COMMENT '清理前的冷存储保留天数（可选）';
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `hot_retention_days` int unsigned NOT NULL DEFAULT '90' COMMENT 'MySQL 热层保留天数';
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `partition_column` varchar(64) NULL COMMENT '分区列，如按月分区的 created_at';

-- settlement_periods 列注释
ALTER TABLE `settlement_periods` MODIFY COLUMN `commission_minor` bigint NOT NULL DEFAULT '0' COMMENT '平台从 GGR 中抽取的分成';
ALTER TABLE `settlement_periods` MODIFY COLUMN `confirmed_by` bigint unsigned NULL COMMENT '确认人（admin_users.id）';
ALTER TABLE `settlement_periods` MODIFY COLUMN `ggr_minor` bigint NOT NULL DEFAULT '0' COMMENT 'GGR = 总下注 − 总派彩';
ALTER TABLE `settlement_periods` MODIFY COLUMN `net_payable_minor` bigint NOT NULL DEFAULT '0' COMMENT '应付净额（平台应收）';
ALTER TABLE `settlement_periods` MODIFY COLUMN `period_type` varchar(16) NOT NULL DEFAULT 'weekly' COMMENT '周期类型：daily/weekly/monthly';
ALTER TABLE `settlement_periods` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'draft' COMMENT '状态：draft/pending_review/confirmed/invoiced/paid';

-- wallet_pending_ops 列注释
ALTER TABLE `wallet_pending_ops` MODIFY COLUMN `op_type` varchar(32) NOT NULL COMMENT '操作类型：win_failed / win_timeout / rollback';
ALTER TABLE `wallet_pending_ops` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT '状态：pending / done / failed';

DELETE FROM schema_migrations WHERE version = '29-repair-column-comments';
