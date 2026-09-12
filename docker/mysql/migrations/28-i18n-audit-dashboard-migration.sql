-- 28: i18n for audit log page and dashboard
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('28-i18n-audit-dashboard', 'Admin i18n for audit logs and dashboard');

INSERT INTO i18n_messages (bundle_id, message_key, default_value, description)
SELECT b.id, k.message_key, k.default_value, k.description
FROM i18n_bundles b
JOIN (
  SELECT 'nav.audit' AS message_key, 'Audit Logs' AS default_value, 'Sidebar nav' AS description
  UNION ALL SELECT 'dash.title', 'Operations Console', 'Dashboard title'
  UNION ALL SELECT 'dash.subtitle', 'Multi-merchant game platform', 'Dashboard subtitle'
  UNION ALL SELECT 'dash.shortcuts', 'Quick Links', 'Dashboard shortcuts'
  UNION ALL SELECT 'dash.merchants', 'Merchants', 'Dashboard stat'
  UNION ALL SELECT 'dash.active', 'Active', 'Dashboard stat sub'
  UNION ALL SELECT 'dash.alerts', 'RTP Alerts', 'Dashboard stat'
  UNION ALL SELECT 'dash.settlements', 'Pending Settlements', 'Dashboard stat'
  UNION ALL SELECT 'dash.breakers', 'Wallet Breakers', 'Dashboard stat'
  UNION ALL SELECT 'err.audit_load', 'Failed to load audit logs', 'Error message'
) k ON b.bundle_code = 'ui.admin'
ON DUPLICATE KEY UPDATE default_value = VALUES(default_value);

INSERT INTO i18n_message_translations (message_id, locale_code, translated_value, status)
SELECT m.id, 'zh-CN', t.translated_value, 1
FROM i18n_messages m
JOIN i18n_bundles b ON b.id = m.bundle_id AND b.bundle_code = 'ui.admin'
JOIN (
  SELECT 'nav.audit' AS message_key, '审计日志' AS translated_value
  UNION ALL SELECT 'dash.title', 'FastGame 运营控制台'
  UNION ALL SELECT 'dash.subtitle', '多商户 · 自研游戏 · 实时风控与对账'
  UNION ALL SELECT 'dash.shortcuts', '快捷入口'
  UNION ALL SELECT 'dash.merchants', '商户总数'
  UNION ALL SELECT 'dash.active', '已启用'
  UNION ALL SELECT 'dash.alerts', 'RTP 告警'
  UNION ALL SELECT 'dash.settlements', '待确认结算'
  UNION ALL SELECT 'dash.breakers', '钱包熔断'
  UNION ALL SELECT 'err.audit_load', '加载审计日志失败'
) t ON t.message_key = m.message_key
ON DUPLICATE KEY UPDATE translated_value = VALUES(translated_value);

SELECT '28-i18n-audit-dashboard-migration applied' AS note;
