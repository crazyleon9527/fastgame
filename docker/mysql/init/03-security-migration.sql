USE fastgame;

-- 已有环境增量迁移 (新环境由 01-init.sql 已包含)

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'merchants' AND COLUMN_NAME = 'private_key_prev'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE merchants ADD COLUMN private_key_prev TEXT NULL COMMENT ''轮换过渡期旧私钥''',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'merchants' AND COLUMN_NAME = 'private_key_prev_expires_at'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE merchants ADD COLUMN private_key_prev_expires_at DATETIME(3) NULL COMMENT ''旧私钥失效时间''',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS risk_blacklist (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  list_type    VARCHAR(32)     NOT NULL COMMENT 'ip / user_id / merchant',
  list_value   VARCHAR(128)    NOT NULL COMMENT '黑名单值',
  reason       VARCHAR(256)    NULL     COMMENT '封禁原因',
  status       TINYINT         NOT NULL DEFAULT 1 COMMENT '1=生效 0=解除',
  expires_at   DATETIME(3)     NULL     COMMENT 'NULL=永久',
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_type_value (list_type, list_value),
  KEY idx_status_expires (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='风控黑名单';
