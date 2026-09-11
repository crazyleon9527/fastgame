-- 26: Repair i18n UTF-8 text corrupted by Windows pipe encoding (stored as ???)
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('26-i18n-utf8-repair', 'Repair i18n Chinese text using UTF-8 hex literals');

UPDATE locales SET native_name = CONVERT(UNHEX('E7AE80E4BD93E4B8ADE69687') USING utf8mb4) WHERE code = 'zh-CN';
UPDATE locales SET native_name = CONVERT(UNHEX('E7B981E9AB94E4B8ADE69687') USING utf8mb4) WHERE code = 'zh-TW';
UPDATE locales SET native_name = CONVERT(UNHEX('E0B984E0B897E0B8A2') USING utf8mb4) WHERE code = 'th-TH';
UPDATE locales SET native_name = CONVERT(UNHEX('5469E1BABF6E67205669E1BB8774') USING utf8mb4) WHERE code = 'vi-VN';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E4BBAAE8A1A8E79B98') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.dashboard';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E59586E688B7E7AEA1E79086') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.merchants';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E6B8B8E6888FE9858DE7BDAE') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.games';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E697A5E7BB93E7AE97') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.settlements';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('52545020E68AA5E8A1A8') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.rtp';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E794A8E688B7E7AEA1E79086') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.users';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E4BF9DE5AD98') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'btn.save';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E58F96E6B688') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'btn.cancel';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E799BBE5BD95') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'btn.login';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E794A8E688B7E5908D') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'label.username';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E5AF86E7A081') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'label.password';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E69CAAE68E88E69D83') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'error.unauthorized';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E69CAAE689BEE588B0') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'error.not_found';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E9A38EE68EA7E9BB91E5908DE58D95') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.blacklist';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('495020E799BDE5908DE58D95') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.whitelist';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('547261636520E8BFBDE8B8AA') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.trace';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('52545020E5918AE8ADA6') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'nav.alerts';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E98080E587BA') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'btn.logout';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('32464120E8AEBEE7BDAE') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'btn.totp';

UPDATE i18n_message_translations t
JOIN i18n_messages m ON m.id = t.message_id
SET t.translated_value = CONVERT(UNHEX('E8BF90E890A5E7AEA1E79086E5908EE58FB0') USING utf8mb4)
WHERE t.locale_code = 'zh-CN' AND m.message_key = 'login.subtitle';

-- Game entity translations
UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E68D95E9B1BCE8BEBEE4BABA') USING utf8mb4)
WHERE entity_type = 'game' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM games WHERE game_code = 'fishing');

UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E5B9B8E8BF90E8BDACE79B98') USING utf8mb4)
WHERE entity_type = 'game' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM games WHERE game_code = 'slot-demo');

UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E781ABE7AEADE59DA0E6AF81') USING utf8mb4)
WHERE entity_type = 'game' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM games WHERE game_code = 'crash-demo');

UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E68D95E9B1BC') USING utf8mb4)
WHERE entity_type = 'category' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM game_categories WHERE code = 'fishing');

UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E88081E8998EE69CBA') USING utf8mb4)
WHERE entity_type = 'category' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM game_categories WHERE code = 'slot');

UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E5B4A9E79B98') USING utf8mb4)
WHERE entity_type = 'category' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM game_categories WHERE code = 'crash');

UPDATE i18n_entity_translations SET translated_value = CONVERT(UNHEX('E6A18CE6B8B8') USING utf8mb4)
WHERE entity_type = 'category' AND field_name = 'name' AND locale_code = 'zh-CN'
  AND entity_id IN (SELECT id FROM game_categories WHERE code = 'table');

SELECT '26-i18n-utf8-repair-migration applied' AS note;
