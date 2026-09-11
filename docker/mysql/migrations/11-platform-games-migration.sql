-- 11: 多商户多游戏平台层 — 游戏目录、商户开通、钱包配置、分成规则
USE fastgame;

-- 迁移版本追踪
CREATE TABLE IF NOT EXISTS schema_migrations (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  version      VARCHAR(64)     NOT NULL COMMENT '迁移版本号',
  description  VARCHAR(256)    NULL,
  applied_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_version (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema migration registry';

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('11-platform-games', 'Platform game catalog, merchant bindings, wallet configs');

-- 币种主数据
CREATE TABLE IF NOT EXISTS currencies (
  code         CHAR(8)         NOT NULL COMMENT 'ISO 或内部码，如 USD CNY USDT',
  name         VARCHAR(64)     NOT NULL,
  symbol       VARCHAR(8)      NULL,
  minor_units  TINYINT         NOT NULL DEFAULT 4 COMMENT '小数位数，对应 money.Scale=10000 时为 4',
  status       TINYINT         NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Currency master';

INSERT INTO currencies (code, name, symbol, minor_units, status) VALUES
  ('USD', 'US Dollar', '$', 4, 1),
  ('CNY', 'Chinese Yuan', '¥', 4, 1),
  ('USDT', 'Tether USD', '₮', 4, 1)
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- 游戏分类
CREATE TABLE IF NOT EXISTS game_categories (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code         VARCHAR(32)     NOT NULL COMMENT 'fishing/slot/crash/table',
  name         VARCHAR(64)     NOT NULL,
  sort_order   INT             NOT NULL DEFAULT 0,
  status       TINYINT         NOT NULL DEFAULT 1,
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_category_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Game categories';

INSERT INTO game_categories (code, name, sort_order) VALUES
  ('fishing', 'Fish Hunter', 10),
  ('slot', 'Slot', 20),
  ('crash', 'Crash', 30),
  ('table', 'Table Game', 40)
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- 平台游戏目录（自研游戏上架）
CREATE TABLE IF NOT EXISTS games (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  game_code       VARCHAR(64)     NOT NULL COMMENT '全局唯一游戏标识',
  name            VARCHAR(128)    NOT NULL,
  category_id     BIGINT UNSIGNED NULL,
  game_type       VARCHAR(32)     NOT NULL COMMENT 'fishing/slot/crash/table',
  default_rtp_ppm BIGINT          NOT NULL DEFAULT 960000 COMMENT '默认 RTP，96%=960000',
  volatility      VARCHAR(16)     NULL COMMENT 'low/medium/high',
  min_bet_minor   BIGINT          NOT NULL DEFAULT 10000 COMMENT '默认最小注 minor units',
  max_bet_minor   BIGINT          NOT NULL DEFAULT 10000000 COMMENT '默认最大注 minor units',
  client_version  VARCHAR(32)     NULL COMMENT '当前客户端版本',
  thumbnail_url   VARCHAR(512)    NULL,
  description     TEXT            NULL,
  status          TINYINT         NOT NULL DEFAULT 1 COMMENT '1=上架 0=下架 2=维护',
  launched_at     DATETIME(3)     NULL,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_game_code (game_code),
  KEY idx_category_status (category_id, status),
  KEY idx_game_type (game_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Platform game catalog';

-- RTP 档位（PAR Sheet 引用）
CREATE TABLE IF NOT EXISTS game_rtp_tiers (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  game_id       BIGINT UNSIGNED NOT NULL,
  tier_code     VARCHAR(32)     NOT NULL COMMENT 'default/high/low 等',
  target_rtp_ppm BIGINT         NOT NULL COMMENT '目标 RTP ppm',
  par_sheet_ref VARCHAR(128)    NULL COMMENT 'PAR 表文件/版本引用',
  weight        INT             NOT NULL DEFAULT 100 COMMENT '随机权重',
  status        TINYINT         NOT NULL DEFAULT 1,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_game_tier (game_id, tier_code),
  KEY idx_game_id (game_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Game RTP tiers';

-- 商户开通游戏
CREATE TABLE IF NOT EXISTS merchant_games (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id     BIGINT UNSIGNED NOT NULL,
  game_id         BIGINT UNSIGNED NOT NULL,
  rtp_tier_code   VARCHAR(32)     NULL COMMENT '覆盖默认 RTP 档位',
  min_bet_minor   BIGINT          NULL COMMENT '覆盖最小注，NULL=用游戏默认',
  max_bet_minor   BIGINT          NULL COMMENT '覆盖最大注',
  sort_order      INT             NOT NULL DEFAULT 0 COMMENT '商户大厅排序',
  status          TINYINT         NOT NULL DEFAULT 1 COMMENT '1=开通 0=关闭',
  opened_at       DATETIME(3)     NULL COMMENT '开通时间',
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_game (merchant_id, game_id),
  KEY idx_merchant_status (merchant_id, status),
  KEY idx_game_id (game_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant game bindings';

-- 商户 Seamless Wallet 配置
CREATE TABLE IF NOT EXISTS merchant_wallet_configs (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id     BIGINT UNSIGNED NOT NULL,
  wallet_type     VARCHAR(32)     NOT NULL DEFAULT 'seamless' COMMENT 'seamless/transfer/mock',
  base_url        VARCHAR(512)    NOT NULL DEFAULT '',
  api_key         VARCHAR(256)    NULL,
  sign_secret     TEXT            NULL COMMENT 'HMAC 密钥，建议加密存储',
  sign_enabled    TINYINT         NOT NULL DEFAULT 1,
  verify_response TINYINT         NOT NULL DEFAULT 1,
  timeout_ms      INT             NOT NULL DEFAULT 5000,
  mock            TINYINT         NOT NULL DEFAULT 0 COMMENT '1=Mock 钱包',
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_wallet (merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant wallet config';

-- 商户启用币种
CREATE TABLE IF NOT EXISTS merchant_currencies (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id   BIGINT UNSIGNED NOT NULL,
  currency_code CHAR(8)         NOT NULL,
  is_default    TINYINT         NOT NULL DEFAULT 0 COMMENT '1=默认币种',
  status        TINYINT         NOT NULL DEFAULT 1,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_currency (merchant_id, currency_code),
  KEY idx_merchant_default (merchant_id, is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant currencies';

-- GGR 分成 / 费率规则
CREATE TABLE IF NOT EXISTS commission_rules (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id     BIGINT UNSIGNED NOT NULL,
  game_id         BIGINT UNSIGNED NULL COMMENT 'NULL=全部游戏',
  rule_type       VARCHAR(32)     NOT NULL DEFAULT 'ggr_share' COMMENT 'ggr_share/fixed_fee',
  rate_ppm        BIGINT          NOT NULL COMMENT '150000=15% 分成',
  min_fee_minor   BIGINT          NOT NULL DEFAULT 0,
  effective_from  DATE            NOT NULL,
  effective_to    DATE            NULL COMMENT 'NULL=长期有效',
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_merchant_effective (merchant_id, effective_from, status),
  KEY idx_game_id (game_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant commission rules';

-- ── 种子：平台游戏 + 绑定 Demo 商户 ──

INSERT INTO games (game_code, name, category_id, game_type, default_rtp_ppm, volatility, min_bet_minor, max_bet_minor, client_version, status, launched_at)
SELECT 'fishing', 'Deep Sea Fishing', c.id, 'fishing', 960000, 'medium', 10000, 10000000, '1.0.0', 1, UTC_TIMESTAMP(3)
FROM game_categories c WHERE c.code = 'fishing'
ON DUPLICATE KEY UPDATE name = VALUES(name), client_version = VALUES(client_version);

INSERT INTO game_rtp_tiers (game_id, tier_code, target_rtp_ppm, par_sheet_ref, weight, status)
SELECT g.id, 'default', 960000, 'fishing-par-v1', 70, 1 FROM games g WHERE g.game_code = 'fishing'
ON DUPLICATE KEY UPDATE target_rtp_ppm = VALUES(target_rtp_ppm);

INSERT INTO game_rtp_tiers (game_id, tier_code, target_rtp_ppm, par_sheet_ref, weight, status)
SELECT g.id, 'high', 965000, 'fishing-par-high-v1', 20, 1 FROM games g WHERE g.game_code = 'fishing'
ON DUPLICATE KEY UPDATE target_rtp_ppm = VALUES(target_rtp_ppm);

INSERT INTO game_rtp_tiers (game_id, tier_code, target_rtp_ppm, par_sheet_ref, weight, status)
SELECT g.id, 'low', 940000, 'fishing-par-low-v1', 10, 1 FROM games g WHERE g.game_code = 'fishing'
ON DUPLICATE KEY UPDATE target_rtp_ppm = VALUES(target_rtp_ppm);

INSERT INTO merchant_games (merchant_id, game_id, rtp_tier_code, status, opened_at)
SELECT m.id, g.id, 'high', 1, UTC_TIMESTAMP(3)
FROM merchants m
JOIN games g ON g.game_code = 'fishing'
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE rtp_tier_code = VALUES(rtp_tier_code);

INSERT INTO merchant_currencies (merchant_id, currency_code, is_default, status)
SELECT m.id, 'USD', 1, 1 FROM merchants m WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE is_default = VALUES(is_default);

INSERT INTO merchant_wallet_configs (merchant_id, wallet_type, base_url, mock, sign_enabled, status)
SELECT m.id, 'mock', '', 1, 0, 1 FROM merchants m WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE mock = VALUES(mock);

INSERT INTO commission_rules (merchant_id, game_id, rule_type, rate_ppm, effective_from, status)
SELECT m.id, g.id, 'ggr_share', 150000, CURDATE(), 1
FROM merchants m
JOIN games g ON g.game_code = 'fishing'
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE rate_ppm = VALUES(rate_ppm);

SELECT '11-platform-games-migration applied' AS note;
