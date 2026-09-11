-- 16: Game client version publishing and merchant upgrade policy
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('16-game-versions', 'Game client versions and merchant upgrade policy');

CREATE TABLE IF NOT EXISTS game_client_versions (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  game_id           BIGINT UNSIGNED NOT NULL,
  version           VARCHAR(32)     NOT NULL COMMENT 'semver e.g. 1.2.0',
  bundle_url        VARCHAR(512)    NULL COMMENT 'CDN path or artifact URL',
  bundle_hash       CHAR(64)        NULL COMMENT 'SHA-256 of bundle',
  changelog         TEXT            NULL,
  min_engine_version VARCHAR(32)    NULL COMMENT 'Cocos/engine requirement',
  status            VARCHAR(16)     NOT NULL DEFAULT 'draft' COMMENT 'draft/published/deprecated',
  published_at      DATETIME(3)     NULL,
  created_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_game_version (game_id, version),
  KEY idx_game_status (game_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Game client version releases';

CREATE TABLE IF NOT EXISTS merchant_game_versions (
  id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id           BIGINT UNSIGNED NOT NULL,
  game_id               BIGINT UNSIGNED NOT NULL,
  client_version_id     BIGINT UNSIGNED NOT NULL,
  min_version_id        BIGINT UNSIGNED NULL COMMENT 'force upgrade below this version',
  force_upgrade         TINYINT         NOT NULL DEFAULT 0,
  rollout_percent       TINYINT         NOT NULL DEFAULT 100 COMMENT 'canary rollout 0-100',
  status                TINYINT         NOT NULL DEFAULT 1,
  effective_from        DATETIME(3)     NULL,
  created_at            DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at            DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_game_version (merchant_id, game_id, client_version_id),
  KEY idx_merchant_game (merchant_id, game_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant game version policy';

-- Seed fishing v1.0.0 for demo
INSERT INTO game_client_versions (game_id, version, bundle_url, status, published_at)
SELECT g.id, '1.0.0', '/game/', 'published', UTC_TIMESTAMP(3)
FROM games g WHERE g.game_code = 'fishing'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO merchant_game_versions (merchant_id, game_id, client_version_id, force_upgrade, rollout_percent, status, effective_from)
SELECT m.id, g.id, cv.id, 0, 100, 1, UTC_TIMESTAMP(3)
FROM merchants m
JOIN games g ON g.game_code = 'fishing'
JOIN game_client_versions cv ON cv.game_id = g.id AND cv.version = '1.0.0'
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE status = VALUES(status);

UPDATE games SET client_version = '1.0.0' WHERE game_code = 'fishing';

SELECT '16-game-versions-migration applied' AS note;
