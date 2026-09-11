-- 27: i18n labels for platform games & merchant lobby admin pages
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('27-i18n-platform-games', 'Admin i18n for platform games and merchant lobby');

INSERT INTO i18n_messages (bundle_id, message_key, default_value, description)
SELECT b.id, k.message_key, k.default_value, k.description
FROM i18n_bundles b
JOIN (
  SELECT 'nav.platform_games' AS message_key, 'Platform Games' AS default_value, 'Sidebar nav' AS description
  UNION ALL SELECT 'nav.merchant_games', 'Merchant Lobby', 'Sidebar nav'
  UNION ALL SELECT 'nav.game_config', 'Game Params', 'Sidebar nav'
) k ON b.bundle_code = 'ui.admin'
ON DUPLICATE KEY UPDATE default_value = VALUES(default_value);

INSERT INTO i18n_message_translations (message_id, locale_code, translated_value, status)
SELECT m.id, 'zh-CN', t.translated_value, 1
FROM i18n_messages m
JOIN i18n_bundles b ON b.id = m.bundle_id AND b.bundle_code = 'ui.admin'
JOIN (
  SELECT 'nav.platform_games' AS message_key, '平台游戏目录' AS translated_value
  UNION ALL SELECT 'nav.merchant_games', '商户游戏大厅'
  UNION ALL SELECT 'nav.game_config', '参数配置'
) t ON t.message_key = m.message_key
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

SELECT '27-i18n-platform-games-migration applied' AS note;
