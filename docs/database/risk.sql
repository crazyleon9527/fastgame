-- =============================================================================
-- 风控模块 (Risk Domain) — RGS / Admin / Consumer 共用
-- 适用引擎: MySQL 8.0+
-- =============================================================================

SET NAMES utf8mb4;

-- -----------------------------------------------------------------------------
-- 1. 风控黑名单 (IP / 用户 / 商户)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `risk_blacklist` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `list_type`    VARCHAR(32)     NOT NULL COMMENT 'ip / user_id / merchant',
  `list_value`   VARCHAR(128)    NOT NULL COMMENT '黑名单值',
  `reason`       VARCHAR(256)    NULL     COMMENT '封禁原因',
  `status`       TINYINT         NOT NULL DEFAULT 1 COMMENT '1=生效 0=解除',
  `expires_at`   DATETIME(3)     NULL     COMMENT 'NULL=永久',
  `created_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_type_value` (`list_type`, `list_value`),
  KEY `idx_status_expires` (`status`, `expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='风控黑名单';

-- -----------------------------------------------------------------------------
-- 2. RTP  watchdog 告警记录
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `risk_alerts` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `alert_type`      VARCHAR(32)     NOT NULL COMMENT 'rtp_threshold 等',
  `scope_type`      VARCHAR(16)     NOT NULL COMMENT 'user | game',
  `scope_value`     VARCHAR(128)    NOT NULL COMMENT '用户ID或游戏代号',
  `merchant_code`   VARCHAR(32)     NOT NULL DEFAULT '',
  `game_code`       VARCHAR(32)     NOT NULL DEFAULT '',
  `rtp_ppm`         BIGINT          NOT NULL COMMENT '实际RTP * 1e6，180%=1800000',
  `total_bet`       BIGINT          NOT NULL,
  `total_win`       BIGINT          NOT NULL,
  `sample_size`     BIGINT          NOT NULL,
  `action_taken`    VARCHAR(64)     NOT NULL COMMENT 'suspend_user 等',
  `status`          VARCHAR(16)     NOT NULL DEFAULT 'open',
  `created_at`      TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_status_created` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='RTP 风控告警';
