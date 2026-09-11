-- 25: Extra admin UI i18n labels
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('25-i18n-admin-extra', 'Additional admin panel i18n labels');

INSERT INTO i18n_messages (bundle_id, message_key, default_value, description)
SELECT b.id, k.message_key, k.default_value, k.description
FROM i18n_bundles b
JOIN (
  SELECT 'nav.blacklist' AS message_key, 'Risk Blacklist' AS default_value, 'Sidebar nav' AS description
  UNION ALL SELECT 'nav.whitelist', 'IP Whitelist', 'Sidebar nav'
  UNION ALL SELECT 'nav.trace', 'Trace Lookup', 'Sidebar nav'
  UNION ALL SELECT 'nav.alerts', 'RTP Alerts', 'Sidebar nav'
  UNION ALL SELECT 'btn.logout', 'Logout', 'Topbar'
  UNION ALL SELECT 'btn.totp', '2FA Setup', 'Topbar'
  UNION ALL SELECT 'login.subtitle', 'Operations Console', 'Login page'
) k ON b.bundle_code = 'ui.admin'
ON DUPLICATE KEY UPDATE default_value = VALUES(default_value);

INSERT INTO i18n_message_translations (message_id, locale_code, translated_value, status)
SELECT m.id, 'zh-CN', t.translated_value, 1
FROM i18n_messages m
JOIN i18n_bundles b ON b.id = m.bundle_id AND b.bundle_code = 'ui.admin'
JOIN (
  SELECT 'nav.blacklist' AS message_key, '风控黑名单' AS translated_value
  UNION ALL SELECT 'nav.whitelist', 'IP 白名单'
  UNION ALL SELECT 'nav.trace', 'Trace 追踪'
  UNION ALL SELECT 'nav.alerts', 'RTP 告警'
  UNION ALL SELECT 'btn.logout', '退出'
  UNION ALL SELECT 'btn.totp', '2FA 设置'
  UNION ALL SELECT 'login.subtitle', '运营管理后台'
) t ON t.message_key = m.message_key
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

SELECT '25-i18n-admin-extra-migration applied' AS note;
