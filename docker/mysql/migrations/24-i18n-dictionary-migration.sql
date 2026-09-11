-- 24: i18n language dictionary — locales, message bundles, entity translations
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('24-i18n-dictionary', 'Language locales, i18n message dictionary, entity translations');

-- Supported locales (BCP 47)
CREATE TABLE IF NOT EXISTS locales (
  code            VARCHAR(16)     NOT NULL COMMENT 'BCP 47 e.g. en-US zh-CN',
  name            VARCHAR(64)     NOT NULL COMMENT 'English display name',
  native_name     VARCHAR(64)     NOT NULL COMMENT 'Name in own language',
  direction       CHAR(3)         NOT NULL DEFAULT 'ltr' COMMENT 'ltr or rtl',
  sort_order      INT             NOT NULL DEFAULT 0,
  is_default      TINYINT         NOT NULL DEFAULT 0 COMMENT 'platform fallback locale',
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (code),
  KEY idx_status_sort (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Supported language locales';

INSERT INTO locales (code, name, native_name, direction, sort_order, is_default, status) VALUES
  ('en-US', 'English (US)', 'English', 'ltr', 10, 1, 1),
  ('zh-CN', 'Chinese (Simplified)', '简体中文', 'ltr', 20, 0, 1),
  ('zh-TW', 'Chinese (Traditional)', '繁體中文', 'ltr', 30, 0, 1),
  ('th-TH', 'Thai', 'ไทย', 'ltr', 40, 0, 1),
  ('vi-VN', 'Vietnamese', 'Tiếng Việt', 'ltr', 50, 0, 1),
  ('id-ID', 'Indonesian', 'Bahasa Indonesia', 'ltr', 60, 0, 1),
  ('pt-BR', 'Portuguese (Brazil)', 'Português (Brasil)', 'ltr', 70, 0, 1)
ON DUPLICATE KEY UPDATE name = VALUES(name), native_name = VALUES(native_name);

-- Message bundles (ui.admin / ui.client / api.errors / game.hints)
CREATE TABLE IF NOT EXISTS i18n_bundles (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  bundle_code     VARCHAR(64)     NOT NULL COMMENT 'namespace e.g. ui.admin',
  description     VARCHAR(256)    NULL,
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_bundle_code (bundle_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='i18n message bundle namespaces';

INSERT INTO i18n_bundles (bundle_code, description) VALUES
  ('ui.admin', 'Admin panel UI strings'),
  ('ui.client', 'Game client / lobby UI strings'),
  ('api.errors', 'API error messages'),
  ('game.hints', 'In-game tutorial and hint text')
ON DUPLICATE KEY UPDATE description = VALUES(description);

-- Dictionary keys with fallback default text (usually en-US)
CREATE TABLE IF NOT EXISTS i18n_messages (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  bundle_id       BIGINT UNSIGNED NOT NULL,
  message_key     VARCHAR(128)    NOT NULL COMMENT 'dot path e.g. nav.merchants',
  default_value   TEXT            NOT NULL COMMENT 'fallback when translation missing',
  description     VARCHAR(256)    NULL COMMENT 'translator context',
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_bundle_key (bundle_id, message_key),
  KEY idx_bundle_status (bundle_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='i18n dictionary keys';

-- Per-locale translated strings
CREATE TABLE IF NOT EXISTS i18n_message_translations (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  message_id      BIGINT UNSIGNED NOT NULL,
  locale_code     VARCHAR(16)     NOT NULL,
  translated_value TEXT           NOT NULL,
  status          TINYINT         NOT NULL DEFAULT 1 COMMENT '1=published 0=draft',
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_message_locale (message_id, locale_code),
  KEY idx_locale_status (locale_code, status),
  CONSTRAINT fk_i18n_msg_tr_locale FOREIGN KEY (locale_code) REFERENCES locales (code),
  CONSTRAINT fk_i18n_msg_tr_message FOREIGN KEY (message_id) REFERENCES i18n_messages (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='i18n message translations';

-- Entity field translations (games, categories, merchants, etc.)
CREATE TABLE IF NOT EXISTS i18n_entity_translations (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  entity_type       VARCHAR(32)     NOT NULL COMMENT 'game/category/merchant/role',
  entity_id         BIGINT UNSIGNED NOT NULL,
  field_name        VARCHAR(32)     NOT NULL COMMENT 'name/description/title',
  locale_code       VARCHAR(16)     NOT NULL,
  translated_value  TEXT            NOT NULL,
  status            TINYINT         NOT NULL DEFAULT 1,
  created_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_entity_field_locale (entity_type, entity_id, field_name, locale_code),
  KEY idx_entity_locale (entity_type, entity_id, locale_code),
  KEY idx_locale_type (locale_code, entity_type, status),
  CONSTRAINT fk_i18n_entity_locale FOREIGN KEY (locale_code) REFERENCES locales (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Entity field translations';

-- Merchant enabled languages
CREATE TABLE IF NOT EXISTS merchant_locales (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id     BIGINT UNSIGNED NOT NULL,
  locale_code     VARCHAR(16)     NOT NULL,
  is_default      TINYINT         NOT NULL DEFAULT 0 COMMENT 'merchant default language',
  sort_order      INT             NOT NULL DEFAULT 0,
  status          TINYINT         NOT NULL DEFAULT 1,
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_locale (merchant_id, locale_code),
  KEY idx_merchant_default (merchant_id, is_default, status),
  CONSTRAINT fk_merchant_locales_locale FOREIGN KEY (locale_code) REFERENCES locales (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant enabled locales';

-- Seed admin UI dictionary (en-US default + zh-CN)
INSERT INTO i18n_messages (bundle_id, message_key, default_value, description)
SELECT b.id, k.message_key, k.default_value, k.description
FROM i18n_bundles b
JOIN (
  SELECT 'nav.dashboard' AS message_key, 'Dashboard' AS default_value, 'Sidebar nav' AS description
  UNION ALL SELECT 'nav.merchants', 'Merchants', 'Sidebar nav'
  UNION ALL SELECT 'nav.games', 'Games', 'Sidebar nav'
  UNION ALL SELECT 'nav.settlements', 'Settlements', 'Sidebar nav'
  UNION ALL SELECT 'nav.rtp', 'RTP Report', 'Sidebar nav'
  UNION ALL SELECT 'nav.users', 'Users', 'Sidebar nav'
  UNION ALL SELECT 'btn.save', 'Save', 'Common button'
  UNION ALL SELECT 'btn.cancel', 'Cancel', 'Common button'
  UNION ALL SELECT 'btn.login', 'Login', 'Login page'
  UNION ALL SELECT 'label.username', 'Username', 'Login form'
  UNION ALL SELECT 'label.password', 'Password', 'Login form'
  UNION ALL SELECT 'error.unauthorized', 'Unauthorized', 'API error'
  UNION ALL SELECT 'error.not_found', 'Not found', 'API error'
) k ON b.bundle_code = 'ui.admin'
ON DUPLICATE KEY UPDATE default_value = VALUES(default_value);

INSERT INTO i18n_message_translations (message_id, locale_code, translated_value, status)
SELECT m.id, 'zh-CN', t.translated_value, 1
FROM i18n_messages m
JOIN i18n_bundles b ON b.id = m.bundle_id AND b.bundle_code = 'ui.admin'
JOIN (
  SELECT 'nav.dashboard' AS message_key, '仪表盘' AS translated_value
  UNION ALL SELECT 'nav.merchants', '商户管理'
  UNION ALL SELECT 'nav.games', '游戏配置'
  UNION ALL SELECT 'nav.settlements', '日结算'
  UNION ALL SELECT 'nav.rtp', 'RTP 报表'
  UNION ALL SELECT 'nav.users', '用户管理'
  UNION ALL SELECT 'btn.save', '保存'
  UNION ALL SELECT 'btn.cancel', '取消'
  UNION ALL SELECT 'btn.login', '登录'
  UNION ALL SELECT 'label.username', '用户名'
  UNION ALL SELECT 'label.password', '密码'
  UNION ALL SELECT 'error.unauthorized', '未授权'
  UNION ALL SELECT 'error.not_found', '未找到'
) t ON t.message_key = m.message_key
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

-- Game name translations
INSERT INTO i18n_entity_translations (entity_type, entity_id, field_name, locale_code, translated_value, status)
SELECT 'game', g.id, 'name', 'zh-CN',
  CASE g.game_code
    WHEN 'fishing' THEN '捕鱼达人'
    WHEN 'slot-demo' THEN '幸运转盘'
    WHEN 'crash-demo' THEN '火箭坠毁'
    ELSE g.name
  END, 1
FROM games g
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

INSERT INTO i18n_entity_translations (entity_type, entity_id, field_name, locale_code, translated_value, status)
SELECT 'game', g.id, 'name', 'zh-TW',
  CASE g.game_code
    WHEN 'fishing' THEN '捕魚達人'
    WHEN 'slot-demo' THEN '幸運轉盤'
    WHEN 'crash-demo' THEN '火箭墜毀'
    ELSE g.name
  END, 1
FROM games g
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

INSERT INTO i18n_entity_translations (entity_type, entity_id, field_name, locale_code, translated_value, status)
SELECT 'game', g.id, 'name', 'th-TH',
  CASE g.game_code
    WHEN 'fishing' THEN 'นักล่าปลา'
    WHEN 'slot-demo' THEN 'สล็อตนำโชค'
    WHEN 'crash-demo' THEN 'จรวดพัง'
    ELSE g.name
  END, 1
FROM games g
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

-- Category name translations
INSERT INTO i18n_entity_translations (entity_type, entity_id, field_name, locale_code, translated_value, status)
SELECT 'category', gc.id, 'name', 'zh-CN',
  CASE gc.code
    WHEN 'fishing' THEN '捕鱼'
    WHEN 'slot' THEN '老虎机'
    WHEN 'crash' THEN '崩盘'
    WHEN 'table' THEN '桌游'
    ELSE gc.name
  END, 1
FROM game_categories gc
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

INSERT INTO i18n_entity_translations (entity_type, entity_id, field_name, locale_code, translated_value, status)
SELECT 'category', gc.id, 'name', 'zh-TW',
  CASE gc.code
    WHEN 'fishing' THEN '捕魚'
    WHEN 'slot' THEN '老虎機'
    WHEN 'crash' THEN '崩盤'
    WHEN 'table' THEN '桌遊'
    ELSE gc.name
  END, 1
FROM game_categories gc
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

-- Demo merchant locales
INSERT INTO merchant_locales (merchant_id, locale_code, is_default, sort_order, status)
SELECT m.id, 'en-US', 1, 10, 1 FROM merchants m WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE is_default = VALUES(is_default);

INSERT INTO merchant_locales (merchant_id, locale_code, is_default, sort_order, status)
SELECT m.id, 'zh-CN', 0, 20, 1 FROM merchants m WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE sort_order = VALUES(sort_order);

INSERT INTO merchant_locales (merchant_id, locale_code, is_default, sort_order, status)
SELECT m.id, 'zh-TW', 0, 30, 1 FROM merchants m WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE sort_order = VALUES(sort_order);

-- Localized lobby view (one row per merchant-game-locale)
DROP VIEW IF EXISTS v_merchant_game_lobby_i18n;

CREATE VIEW v_merchant_game_lobby_i18n AS
SELECT
  l.merchant_id,
  l.merchant_code,
  ml.locale_code,
  l.game_id,
  l.game_code,
  COALESCE(gn.translated_value, l.game_name) AS game_name,
  COALESCE(cn.translated_value, l.category_name) AS category_name,
  l.game_type,
  l.category_code,
  l.lobby_sort,
  l.effective_min_bet_minor,
  l.effective_max_bet_minor,
  l.client_version,
  l.thumbnail_url,
  l.tier_rtp_ppm
FROM v_merchant_game_lobby l
JOIN merchant_locales ml ON ml.merchant_id = l.merchant_id AND ml.status = 1
LEFT JOIN i18n_entity_translations gn
  ON gn.entity_type = 'game' AND gn.entity_id = l.game_id
  AND gn.field_name = 'name' AND gn.locale_code = ml.locale_code AND gn.status = 1
LEFT JOIN game_categories gc ON gc.code = l.category_code
LEFT JOIN i18n_entity_translations cn
  ON cn.entity_type = 'category' AND cn.entity_id = gc.id
  AND cn.field_name = 'name' AND cn.locale_code = ml.locale_code AND cn.status = 1;

SELECT '24-i18n-dictionary-migration applied' AS note;
