-- 29-repair-column-comments-migration.sql
-- 修复被错误 charset 毁掉的表/列注释（ASCII 问号 / 双重编码乱码）。
-- 注释内容取自 docker/mysql/migrations/comment_manifest.json（由迁移文件提取）。
-- 列定义由 information_schema 重建，只替换 COMMENT，类型/默认值/extra 不变。
-- 由 scripts/gen_comment_repair.ps1 生成，请勿手工编辑。
USE fastgame;

SET NAMES utf8mb4;

-- 表注释
ALTER TABLE `api_rate_limits` COMMENT = 'Merchant API rate limits';
ALTER TABLE `audit_logs` COMMENT = 'Admin audit log';
ALTER TABLE `commission_rules` COMMENT = 'Merchant commission rules';
ALTER TABLE `currencies` COMMENT = 'Currency master';
ALTER TABLE `game_categories` COMMENT = 'Game categories';
ALTER TABLE `game_client_versions` COMMENT = 'Game client version releases';
ALTER TABLE `game_maintenance_windows` COMMENT = 'Game maintenance schedule';
ALTER TABLE `game_round_replay` COMMENT = 'Deterministic replay inputs';
ALTER TABLE `game_rtp_tiers` COMMENT = 'Game RTP tiers';
ALTER TABLE `game_sessions` COMMENT = 'Player session registry';
ALTER TABLE `games` COMMENT = 'Platform game catalog';
ALTER TABLE `i18n_bundles` COMMENT = 'i18n message bundle namespaces';
ALTER TABLE `i18n_entity_translations` COMMENT = 'Entity field translations';
ALTER TABLE `i18n_message_translations` COMMENT = 'i18n message translations';
ALTER TABLE `i18n_messages` COMMENT = 'i18n dictionary keys';
ALTER TABLE `locales` COMMENT = 'Supported language locales';
ALTER TABLE `merchant_currencies` COMMENT = 'Merchant currencies';
ALTER TABLE `merchant_game_versions` COMMENT = 'Merchant game version policy';
ALTER TABLE `merchant_games` COMMENT = 'Merchant game bindings';
ALTER TABLE `merchant_locales` COMMENT = 'Merchant enabled locales';
ALTER TABLE `merchant_settlement_lines` COMMENT = 'Settlement period line items';
ALTER TABLE `merchant_wallet_configs` COMMENT = 'Merchant wallet config';
ALTER TABLE `merchant_webhooks` COMMENT = 'Merchant outbound webhooks';
ALTER TABLE `player_merchant_profiles` COMMENT = 'Player profile per merchant';
ALTER TABLE `schema_archive_policies` COMMENT = 'Table archival retention policies';
ALTER TABLE `schema_migrations` COMMENT = 'Schema migration registry';
ALTER TABLE `settlement_periods` COMMENT = 'Merchant settlement billing period';

-- api_rate_limits 列注释
ALTER TABLE `api_rate_limits` MODIFY COLUMN `burst` int unsigned NULL COMMENT 'token bucket burst';
ALTER TABLE `api_rate_limits` MODIFY COLUMN `limit_scope` varchar(32) NOT NULL DEFAULT 'global' COMMENT 'global/bet/session/ip';
ALTER TABLE `api_rate_limits` MODIFY COLUMN `limit_type` varchar(32) NOT NULL DEFAULT 'rps' COMMENT 'rps/daily_quota/concurrent';

-- audit_logs 列注释
ALTER TABLE `audit_logs` MODIFY COLUMN `action` varchar(64) NOT NULL COMMENT 'create_merchant/rotate_key/confirm_settlement etc';
ALTER TABLE `audit_logs` MODIFY COLUMN `detail` json NULL COMMENT 'request snapshot / diff';
ALTER TABLE `audit_logs` MODIFY COLUMN `resource_type` varchar(64) NOT NULL COMMENT 'merchant/game/config/user/settlement';

-- commission_rules 列注释
ALTER TABLE `commission_rules` MODIFY COLUMN `rule_type` varchar(32) NOT NULL DEFAULT 'ggr_share' COMMENT 'ggr_share/fixed_fee';

-- game_categories 列注释
ALTER TABLE `game_categories` MODIFY COLUMN `code` varchar(32) NOT NULL COMMENT 'fishing/slot/crash/table';

-- game_client_versions 列注释
ALTER TABLE `game_client_versions` MODIFY COLUMN `bundle_hash` char(64) NULL COMMENT 'SHA-256 of bundle';
ALTER TABLE `game_client_versions` MODIFY COLUMN `bundle_url` varchar(512) NULL COMMENT 'CDN path or artifact URL';
ALTER TABLE `game_client_versions` MODIFY COLUMN `min_engine_version` varchar(32) NULL COMMENT 'Cocos/engine requirement';
ALTER TABLE `game_client_versions` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'draft' COMMENT 'draft/published/deprecated';
ALTER TABLE `game_client_versions` MODIFY COLUMN `version` varchar(32) NOT NULL COMMENT 'semver e.g. 1.2.0';

-- game_maintenance_windows 列注释
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `created_by` bigint unsigned NULL COMMENT 'admin_users.id';
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `game_id` bigint unsigned NULL COMMENT 'NULL=all games';
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `merchant_id` bigint unsigned NULL COMMENT 'NULL=platform-wide';
ALTER TABLE `game_maintenance_windows` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'scheduled' COMMENT 'scheduled/active/completed/cancelled';

-- game_round_replay 列注释
ALTER TABLE `game_round_replay` MODIFY COLUMN `round_id` varchar(64) NOT NULL COMMENT 'Nonce / Round ID';

-- game_sessions 列注释
ALTER TABLE `game_sessions` MODIFY COLUMN `client_ip` varchar(45) NULL COMMENT 'IPv4 or IPv6';
ALTER TABLE `game_sessions` MODIFY COLUMN `round_count` int unsigned NOT NULL DEFAULT '0' COMMENT 'bet rounds in this session';
ALTER TABLE `game_sessions` MODIFY COLUMN `session_token_hash` char(64) NOT NULL COMMENT 'SHA-256 of session token, never store raw token';
ALTER TABLE `game_sessions` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'active' COMMENT 'active/expired/revoked';

-- games 列注释
ALTER TABLE `games` MODIFY COLUMN `game_type` varchar(32) NOT NULL COMMENT 'fishing/slot/crash/table';
ALTER TABLE `games` MODIFY COLUMN `volatility` varchar(16) NULL COMMENT 'low/medium/high';

-- i18n_bundles 列注释
ALTER TABLE `i18n_bundles` MODIFY COLUMN `bundle_code` varchar(64) NOT NULL COMMENT 'namespace e.g. ui.admin';

-- i18n_entity_translations 列注释
ALTER TABLE `i18n_entity_translations` MODIFY COLUMN `entity_type` varchar(32) NOT NULL COMMENT 'game/category/merchant/role';
ALTER TABLE `i18n_entity_translations` MODIFY COLUMN `field_name` varchar(32) NOT NULL COMMENT 'name/description/title';

-- i18n_message_translations 列注释
ALTER TABLE `i18n_message_translations` MODIFY COLUMN `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=published 0=draft';

-- i18n_messages 列注释
ALTER TABLE `i18n_messages` MODIFY COLUMN `default_value` text NOT NULL COMMENT 'fallback when translation missing';
ALTER TABLE `i18n_messages` MODIFY COLUMN `description` varchar(256) NULL COMMENT 'translator context';
ALTER TABLE `i18n_messages` MODIFY COLUMN `message_key` varchar(128) NOT NULL COMMENT 'dot path e.g. nav.merchants';

-- locales 列注释
ALTER TABLE `locales` MODIFY COLUMN `code` varchar(16) NOT NULL COMMENT 'BCP 47 e.g. en-US zh-CN';
ALTER TABLE `locales` MODIFY COLUMN `direction` char(3) NOT NULL DEFAULT 'ltr' COMMENT 'ltr or rtl';
ALTER TABLE `locales` MODIFY COLUMN `is_default` tinyint NOT NULL DEFAULT '0' COMMENT 'platform fallback locale';
ALTER TABLE `locales` MODIFY COLUMN `name` varchar(64) NOT NULL COMMENT 'English display name';
ALTER TABLE `locales` MODIFY COLUMN `native_name` varchar(64) NOT NULL COMMENT 'Name in own language';

-- merchant_game_versions 列注释
ALTER TABLE `merchant_game_versions` MODIFY COLUMN `min_version_id` bigint unsigned NULL COMMENT 'force upgrade below this version';
ALTER TABLE `merchant_game_versions` MODIFY COLUMN `rollout_percent` tinyint NOT NULL DEFAULT '100' COMMENT 'canary rollout 0-100';

-- merchant_locales 列注释
ALTER TABLE `merchant_locales` MODIFY COLUMN `is_default` tinyint NOT NULL DEFAULT '0' COMMENT 'merchant default language';

-- merchant_settlement_lines 列注释
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `amount_minor` bigint NOT NULL DEFAULT '0' COMMENT 'signed line amount';
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `commission_rate_ppm` bigint NULL COMMENT 'snapshot of rate at close';
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `daily_settlement_id` bigint unsigned NULL COMMENT 'link to daily_settlements.id';
ALTER TABLE `merchant_settlement_lines` MODIFY COLUMN `ref_date` date NULL COMMENT 'for daily_aggregate lines';

-- merchant_wallet_configs 列注释
ALTER TABLE `merchant_wallet_configs` MODIFY COLUMN `wallet_type` varchar(32) NOT NULL DEFAULT 'seamless' COMMENT 'seamless/transfer/mock';

-- merchant_webhooks 列注释
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `event_type` varchar(64) NOT NULL COMMENT 'settlement.confirmed/bigwin/risk.alert';
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `headers` json NULL COMMENT 'extra HTTP headers';
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `secret` varchar(256) NULL COMMENT 'HMAC signing secret';
ALTER TABLE `merchant_webhooks` MODIFY COLUMN `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=active 0=disabled';

-- pending_transactions 列注释
ALTER TABLE `pending_transactions` MODIFY COLUMN `expected_action` varchar(16) NOT NULL COMMENT 'settle_win|rollback_bet|none';
ALTER TABLE `pending_transactions` MODIFY COLUMN `phase` varchar(32) NOT NULL COMMENT 'bet_debited|win_pending|settled|orphan';
ALTER TABLE `pending_transactions` MODIFY COLUMN `round_id` varchar(64) NOT NULL COMMENT 'Round / Nonce';
ALTER TABLE `pending_transactions` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT 'pending|done|failed';

-- player_merchant_profiles 列注释
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `bet_limit_override` json NULL COMMENT '{"min":10000,"max":5000000}';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `external_user_ref` varchar(128) NULL COMMENT 'optional merchant-side string id';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `risk_level` tinyint NOT NULL DEFAULT '0' COMMENT '0=normal, 9=blocked';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `status` tinyint NOT NULL DEFAULT '1' COMMENT '1=active 0=suspended';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `tags` json NULL COMMENT '["high_roller","test"]';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `user_id` bigint unsigned NOT NULL COMMENT 'player id in merchant system';
ALTER TABLE `player_merchant_profiles` MODIFY COLUMN `vip_level` tinyint NOT NULL DEFAULT '0' COMMENT '0=normal, higher=better';

-- risk_alerts 列注释
ALTER TABLE `risk_alerts` MODIFY COLUMN `scope_type` varchar(16) NOT NULL COMMENT 'user | game';

-- risk_blacklist 列注释
ALTER TABLE `risk_blacklist` MODIFY COLUMN `list_type` varchar(32) NOT NULL COMMENT 'ip / user_id / merchant';

-- schema_archive_policies 列注释
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `archive_strategy` varchar(32) NOT NULL DEFAULT 'delete' COMMENT 'delete/export_to_s3/partition_drop';
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `cold_retention_days` int unsigned NULL COMMENT 'optional cold storage before purge';
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `hot_retention_days` int unsigned NOT NULL DEFAULT '90' COMMENT 'keep in MySQL hot tier';
ALTER TABLE `schema_archive_policies` MODIFY COLUMN `partition_column` varchar(64) NULL COMMENT 'e.g. created_at for monthly partitions';

-- settlement_periods 列注释
ALTER TABLE `settlement_periods` MODIFY COLUMN `commission_minor` bigint NOT NULL DEFAULT '0' COMMENT 'platform share from GGR';
ALTER TABLE `settlement_periods` MODIFY COLUMN `confirmed_by` bigint unsigned NULL COMMENT 'admin_users.id';
ALTER TABLE `settlement_periods` MODIFY COLUMN `ggr_minor` bigint NOT NULL DEFAULT '0' COMMENT 'total_bet - total_win';
ALTER TABLE `settlement_periods` MODIFY COLUMN `net_payable_minor` bigint NOT NULL DEFAULT '0' COMMENT 'amount due (platform receivable)';
ALTER TABLE `settlement_periods` MODIFY COLUMN `period_type` varchar(16) NOT NULL DEFAULT 'weekly' COMMENT 'daily/weekly/monthly';
ALTER TABLE `settlement_periods` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'draft' COMMENT 'draft/pending_review/confirmed/invoiced/paid';

-- wallet_pending_ops 列注释
ALTER TABLE `wallet_pending_ops` MODIFY COLUMN `op_type` varchar(32) NOT NULL COMMENT 'win_failed / win_timeout / rollback';
ALTER TABLE `wallet_pending_ops` MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT 'pending / done / failed';

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('29-repair-column-comments', 'Restore Chinese column comments damaged by wrong charset');

SELECT '29-repair-column-comments applied' AS note;
