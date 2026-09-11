-- 15: API rate limits and merchant webhooks
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('15-api-governance', 'API rate limits and merchant webhooks');

CREATE TABLE IF NOT EXISTS api_rate_limits (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id     BIGINT UNSIGNED NOT NULL,
  limit_scope     VARCHAR(32)     NOT NULL DEFAULT 'global' COMMENT 'global/bet/session/ip',
  limit_type      VARCHAR(32)     NOT NULL DEFAULT 'rps' COMMENT 'rps/daily_quota/concurrent',
  limit_value     INT UNSIGNED    NOT NULL,
  window_seconds  INT UNSIGNED    NOT NULL DEFAULT 1,
  burst           INT UNSIGNED    NULL COMMENT 'token bucket burst',
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_scope_type (merchant_id, limit_scope, limit_type),
  KEY idx_merchant_status (merchant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant API rate limits';

CREATE TABLE IF NOT EXISTS merchant_webhooks (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id     BIGINT UNSIGNED NOT NULL,
  event_type      VARCHAR(64)     NOT NULL COMMENT 'settlement.confirmed/bigwin/risk.alert',
  target_url      VARCHAR(512)    NOT NULL,
  secret          VARCHAR(256)    NULL COMMENT 'HMAC signing secret',
  headers         JSON            NULL COMMENT 'extra HTTP headers',
  retry_max       TINYINT         NOT NULL DEFAULT 5,
  timeout_ms      INT             NOT NULL DEFAULT 5000,
  status          TINYINT         NOT NULL DEFAULT 1 COMMENT '1=active 0=disabled',
  last_triggered_at DATETIME(3)   NULL,
  last_status_code  SMALLINT      NULL,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_event (merchant_id, event_type),
  KEY idx_merchant_status (merchant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant outbound webhooks';

-- Default rate limit for demo merchant
INSERT INTO api_rate_limits (merchant_id, limit_scope, limit_type, limit_value, window_seconds, burst, status)
SELECT m.id, 'global', 'rps', 20, 1, 40, 1 FROM merchants m WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE limit_value = VALUES(limit_value);

SELECT '15-api-governance-migration applied' AS note;
