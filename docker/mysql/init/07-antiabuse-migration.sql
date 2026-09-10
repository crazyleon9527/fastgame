USE fastgame;

CREATE TABLE IF NOT EXISTS risk_alerts (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    alert_type      VARCHAR(32)  NOT NULL,
    scope_type      VARCHAR(16)  NOT NULL COMMENT 'user | game',
    scope_value     VARCHAR(128) NOT NULL,
    merchant_code   VARCHAR(32)  NOT NULL DEFAULT '',
    game_code       VARCHAR(32)  NOT NULL DEFAULT '',
    rtp_ppm         BIGINT       NOT NULL COMMENT '实际RTP * 1e6，180%=1800000',
    total_bet       BIGINT       NOT NULL,
    total_win       BIGINT       NOT NULL,
    sample_size     BIGINT       NOT NULL,
    action_taken    VARCHAR(64)  NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'open',
    created_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    KEY idx_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_secret'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_secret VARCHAR(64) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_enabled'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_enabled TINYINT NOT NULL DEFAULT 0',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
