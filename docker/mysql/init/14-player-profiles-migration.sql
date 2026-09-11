-- 14: Player profiles per merchant (VIP, tags, limit overrides)
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('14-player-profiles', 'Player merchant profiles with VIP and limits');

CREATE TABLE IF NOT EXISTS player_merchant_profiles (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id         BIGINT UNSIGNED NOT NULL,
  user_id             BIGINT UNSIGNED NOT NULL COMMENT 'player id in merchant system',
  external_user_ref   VARCHAR(128)    NULL COMMENT 'optional merchant-side string id',
  display_name        VARCHAR(128)    NULL,
  vip_level           TINYINT         NOT NULL DEFAULT 0 COMMENT '0=normal, higher=better',
  risk_level          TINYINT         NOT NULL DEFAULT 0 COMMENT '0=normal, 9=blocked',
  tags                JSON            NULL COMMENT '["high_roller","test"]',
  bet_limit_override  JSON            NULL COMMENT '{"min":10000,"max":5000000}',
  currency_code       CHAR(8)         NOT NULL DEFAULT 'USD',
  lifetime_bet_minor  BIGINT          NOT NULL DEFAULT 0,
  lifetime_win_minor  BIGINT          NOT NULL DEFAULT 0,
  lifetime_rounds     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  last_played_at      DATETIME(3)     NULL,
  status              TINYINT         NOT NULL DEFAULT 1 COMMENT '1=active 0=suspended',
  created_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_user (merchant_id, user_id),
  KEY idx_merchant_vip (merchant_id, vip_level, status),
  KEY idx_merchant_risk (merchant_id, risk_level),
  KEY idx_last_played (merchant_id, last_played_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Player profile per merchant';

SELECT '14-player-profiles-migration applied' AS note;
