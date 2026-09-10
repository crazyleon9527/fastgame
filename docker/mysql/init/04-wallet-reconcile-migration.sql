USE fastgame;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'merchants' AND COLUMN_NAME = 'allowed_ips'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE merchants ADD COLUMN allowed_ips JSON NULL COMMENT ''聚合器报备公网 IP 白名单''',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS wallet_pending_ops (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  round_id      VARCHAR(128)    NOT NULL,
  merchant_code VARCHAR(64)     NOT NULL,
  user_id       BIGINT UNSIGNED NOT NULL,
  op_type       VARCHAR(32)     NOT NULL COMMENT 'win_failed / win_timeout / rollback',
  bet_amount    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  win_amount    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  status        VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending / done / failed',
  retry_count   INT             NOT NULL DEFAULT 0,
  last_error    TEXT            NULL,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_round_op (round_id, op_type),
  KEY idx_status_updated (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='钱包待对账';
