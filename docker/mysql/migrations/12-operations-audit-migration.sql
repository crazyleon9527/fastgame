-- 12: Operations & audit — player sessions, admin audit trail, maintenance windows
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('12-operations-audit', 'Game sessions, admin audit logs, maintenance windows');

-- Player session registry (Redis is hot path; MySQL for lifecycle audit & reconciliation)
CREATE TABLE IF NOT EXISTS game_sessions (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  session_token_hash CHAR(64)       NOT NULL COMMENT 'SHA-256 of session token, never store raw token',
  merchant_id       BIGINT UNSIGNED NOT NULL,
  merchant_code     VARCHAR(64)     NOT NULL,
  user_id           BIGINT UNSIGNED NOT NULL,
  game_id           BIGINT UNSIGNED NULL,
  game_code         VARCHAR(64)     NOT NULL,
  currency_code     CHAR(8)         NOT NULL DEFAULT 'USD',
  client_seed       VARCHAR(128)    NULL,
  server_seed_hash  VARCHAR(128)    NULL,
  client_ip         VARCHAR(45)     NULL COMMENT 'IPv4 or IPv6',
  user_agent        VARCHAR(512)    NULL,
  status            VARCHAR(16)     NOT NULL DEFAULT 'active' COMMENT 'active/expired/revoked',
  round_count       INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT 'bet rounds in this session',
  expires_at        DATETIME(3)     NOT NULL,
  last_active_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_session_token_hash (session_token_hash),
  KEY idx_merchant_user_active (merchant_id, user_id, status),
  KEY idx_game_status_expires (game_code, status, expires_at),
  KEY idx_expires_status (expires_at, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Player session registry';

-- Admin operation audit trail
CREATE TABLE IF NOT EXISTS audit_logs (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  admin_user_id   BIGINT UNSIGNED NULL,
  username        VARCHAR(64)     NOT NULL DEFAULT '',
  role_name       VARCHAR(32)     NOT NULL DEFAULT '',
  action          VARCHAR(64)     NOT NULL COMMENT 'create_merchant/rotate_key/confirm_settlement etc',
  resource_type   VARCHAR(64)     NOT NULL DEFAULT '' COMMENT 'merchant/game/config/user/settlement',
  resource_id     VARCHAR(128)    NOT NULL DEFAULT '',
  http_method     VARCHAR(8)      NULL,
  request_path    VARCHAR(256)    NULL,
  detail          JSON            NULL COMMENT 'request snapshot / diff',
  client_ip       VARCHAR(45)     NULL,
  status_code     SMALLINT        NULL,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_admin_user_created (admin_user_id, created_at),
  KEY idx_action_created (action, created_at),
  KEY idx_resource (resource_type, resource_id),
  KEY idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Admin audit log';

-- Game maintenance windows (platform-wide or per-merchant)
CREATE TABLE IF NOT EXISTS game_maintenance_windows (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  game_id         BIGINT UNSIGNED NULL COMMENT 'NULL=all games',
  merchant_id     BIGINT UNSIGNED NULL COMMENT 'NULL=platform-wide',
  title           VARCHAR(128)    NOT NULL,
  reason          VARCHAR(512)    NULL,
  starts_at       DATETIME(3)     NOT NULL,
  ends_at         DATETIME(3)     NOT NULL,
  status          VARCHAR(16)     NOT NULL DEFAULT 'scheduled' COMMENT 'scheduled/active/completed/cancelled',
  created_by      BIGINT UNSIGNED NULL COMMENT 'admin_users.id',
  cancelled_by    BIGINT UNSIGNED NULL,
  cancelled_at    DATETIME(3)     NULL,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_game_merchant_starts (game_id, merchant_id, starts_at),
  KEY idx_status_window (status, starts_at, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Game maintenance schedule';

-- Index optimization: merchant_games lookup by game + status (hot path for lobby)
SET @idx_exists = (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'merchant_games' AND INDEX_NAME = 'idx_game_status_sort'
);
SET @sql = IF(@idx_exists = 0,
  'CREATE INDEX idx_game_status_sort ON merchant_games (game_id, status, sort_order)',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SELECT '12-operations-audit-migration applied' AS note;
