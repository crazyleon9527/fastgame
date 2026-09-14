-- =============================================================================
-- 平台运行时表 (Platform / RGS Legacy Runtime)
-- RGS、Admin、Rollback 服务直接依赖的 MySQL 表（与 biz 域表并存）
-- 适用引擎: MySQL 8.0+
-- =============================================================================

SET NAMES utf8mb4;

-- -----------------------------------------------------------------------------
-- 1. 商户主体 (RGS 旧表，merchantId 字符串对接)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `merchants` (
  `id`                         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `merchant_code`              VARCHAR(64)     NOT NULL COMMENT '商户唯一编码',
  `name`                       VARCHAR(128)    NOT NULL COMMENT '商户名称',
  `public_key`                 TEXT            NULL     COMMENT 'API 公钥',
  `private_key`                TEXT            NULL     COMMENT 'API 私钥 / HMAC 签名密钥',
  `private_key_prev`           TEXT            NULL     COMMENT '轮换过渡期旧私钥',
  `private_key_prev_expires_at` DATETIME(3)    NULL     COMMENT '旧私钥失效时间',
  `allowed_ips`                JSON            NULL     COMMENT '聚合器报备公网 IP 白名单',
  `status`                     TINYINT         NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
  `created_at`                 DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`                 DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_merchant_code` (`merchant_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='RGS 商户主体';

-- 已有 merchants 表增量补列
SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'merchants' AND COLUMN_NAME = 'private_key_prev'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE merchants ADD COLUMN private_key_prev TEXT NULL COMMENT ''轮换过渡期旧私钥''',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'merchants' AND COLUMN_NAME = 'private_key_prev_expires_at'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE merchants ADD COLUMN private_key_prev_expires_at DATETIME(3) NULL COMMENT ''旧私钥失效时间''',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'merchants' AND COLUMN_NAME = 'allowed_ips'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE merchants ADD COLUMN allowed_ips JSON NULL COMMENT ''聚合器报备公网 IP 白名单''',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- -----------------------------------------------------------------------------
-- 2. 游戏参数字典 (多档 RTP / 限额)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `game_configs` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `merchant_id`  BIGINT UNSIGNED NOT NULL,
  `game_code`    VARCHAR(64)     NOT NULL COMMENT '游戏标识',
  `config_key`   VARCHAR(128)    NOT NULL COMMENT '配置键',
  `config_value` JSON            NOT NULL COMMENT '配置值',
  `rtp_tier`     VARCHAR(32)     NULL     COMMENT 'RTP 档位',
  `status`       TINYINT         NOT NULL DEFAULT 1,
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_merchant_game_key` (`merchant_id`, `game_code`, `config_key`),
  KEY `idx_merchant_game` (`merchant_id`, `game_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='游戏参数配置';

-- -----------------------------------------------------------------------------
-- 3. RBAC
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `roles` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`        VARCHAR(64)     NOT NULL,
  `description` VARCHAR(256)    NULL,
  `created_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色';

CREATE TABLE IF NOT EXISTS `admin_users` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username`      VARCHAR(64)     NOT NULL,
  `password_hash` VARCHAR(256)    NOT NULL,
  `role_id`       BIGINT UNSIGNED NOT NULL,
  `status`        TINYINT         NOT NULL DEFAULT 1,
  `totp_secret`   VARCHAR(64)     NULL,
  `totp_enabled`  TINYINT         NOT NULL DEFAULT 0,
  `totp_recovery_hashes` JSON     NULL,
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理后台用户';

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_secret'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_secret VARCHAR(64) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_enabled'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_enabled TINYINT NOT NULL DEFAULT 0',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_recovery_hashes'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_recovery_hashes JSON NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- -----------------------------------------------------------------------------
-- 4. 每日结算对账单
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `daily_settlements` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `merchant_id`  BIGINT UNSIGNED NOT NULL,
  `settle_date`  DATE            NOT NULL COMMENT '结算日期 UTC',
  `total_bet`    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  `total_win`    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  `total_rounds` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `status`       TINYINT         NOT NULL DEFAULT 0 COMMENT '0=待确认 1=已确认',
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_merchant_date` (`merchant_id`, `settle_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='每日结算对账单';

-- -----------------------------------------------------------------------------
-- 5. 钱包待对账 (Rollback Worker)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `wallet_pending_ops` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `round_id`      VARCHAR(128)    NOT NULL,
  `merchant_code` VARCHAR(64)     NOT NULL,
  `user_id`       VARCHAR(64)     NOT NULL COMMENT '下游玩家唯一ID',
  `op_type`       VARCHAR(32)     NOT NULL COMMENT 'win_failed / win_timeout / rollback',
  `bet_amount`    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  `win_amount`    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  `status`        VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending / done / failed',
  `retry_count`   INT             NOT NULL DEFAULT 0,
  `last_error`    TEXT            NULL,
  `created_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_round_op` (`round_id`, `op_type`),
  KEY `idx_status_updated` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='钱包待对账';

-- -----------------------------------------------------------------------------
-- 6. 孤儿注单补偿 (Pending Transaction Reconciler)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `pending_transactions` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `trace_id`          VARCHAR(64)     NOT NULL COMMENT '全链路 TraceID',
  `round_id`          VARCHAR(64)     NOT NULL COMMENT 'Round / Nonce',
  `merchant_code`     VARCHAR(32)     NOT NULL,
  `user_id`           VARCHAR(64)     NOT NULL COMMENT '下游玩家唯一ID',
  `game_code`         VARCHAR(32)     NOT NULL,
  `phase`             VARCHAR(32)     NOT NULL COMMENT 'bet_debited|win_pending|settled|orphan',
  `status`            VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|done|failed',
  `bet_amount`        DECIMAL(18,4)   NOT NULL,
  `win_amount`        DECIMAL(18,4)   NOT NULL DEFAULT 0,
  `expected_action`   VARCHAR(16)     NOT NULL COMMENT 'settle_win|rollback_bet|none',
  `wallet_bet_status` VARCHAR(16)     NOT NULL DEFAULT 'confirmed',
  `wallet_win_status` VARCHAR(16)     NOT NULL DEFAULT 'unknown',
  `retry_count`       INT             NOT NULL DEFAULT 0,
  `last_error`        VARCHAR(512)    NULL,
  `created_at`        TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`        TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_round_id` (`round_id`),
  KEY `idx_status_created` (`status`, `created_at`),
  KEY `idx_trace_id` (`trace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='孤儿注单自动对账补偿';

-- -----------------------------------------------------------------------------
-- 7. 确定性回放输入
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `game_round_replay` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `round_id`      VARCHAR(64)     NOT NULL COMMENT 'Nonce / Round ID',
  `merchant_code` VARCHAR(32)     NOT NULL,
  `user_id`       VARCHAR(64)     NOT NULL COMMENT '下游玩家唯一ID',
  `game_code`     VARCHAR(32)     NOT NULL,
  `server_seed`   VARCHAR(128)    NOT NULL,
  `client_seed`   VARCHAR(128)    NOT NULL,
  `nonce`         VARCHAR(64)     NOT NULL,
  `bet_amount`    DECIMAL(18,4)   NOT NULL,
  `sequence_id`   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at`    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_round_id` (`round_id`),
  KEY `idx_user_created` (`user_id`, `created_at`),
  KEY `idx_merchant_created` (`merchant_code`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Deterministic replay inputs';
