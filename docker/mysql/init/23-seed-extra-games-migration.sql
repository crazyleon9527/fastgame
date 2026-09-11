-- 23: Seed slot and crash placeholder games for multi-game testing
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('23-seed-extra-games', 'Seed slot/crash placeholder games and demo merchant bindings');

INSERT INTO games (game_code, name, category_id, game_type, default_rtp_ppm, volatility, min_bet_minor, max_bet_minor, client_version, status, launched_at)
SELECT 'slot-demo', 'Lucky Spin Demo', gc.id, 'slot', 965000, 'medium', 10000, 5000000, '0.1.0', 1, UTC_TIMESTAMP(3)
FROM game_categories gc WHERE gc.code = 'slot'
ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status);

INSERT INTO games (game_code, name, category_id, game_type, default_rtp_ppm, volatility, min_bet_minor, max_bet_minor, client_version, status, launched_at)
SELECT 'crash-demo', 'Rocket Crash Demo', gc.id, 'crash', 970000, 'high', 10000, 2000000, '0.1.0', 1, UTC_TIMESTAMP(3)
FROM game_categories gc WHERE gc.code = 'crash'
ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status);

INSERT INTO game_rtp_tiers (game_id, tier_code, target_rtp_ppm, par_sheet_ref, weight, status)
SELECT g.id, 'default', g.default_rtp_ppm, CONCAT(g.game_code, '/par-v1.json'), 100, 1
FROM games g WHERE g.game_code IN ('slot-demo', 'crash-demo')
ON DUPLICATE KEY UPDATE target_rtp_ppm = VALUES(target_rtp_ppm);

INSERT INTO merchant_games (merchant_id, game_id, rtp_tier_code, sort_order, status, opened_at)
SELECT m.id, g.id, 'default',
  CASE g.game_code WHEN 'slot-demo' THEN 20 WHEN 'crash-demo' THEN 30 ELSE 99 END,
  1, UTC_TIMESTAMP(3)
FROM merchants m
JOIN games g ON g.game_code IN ('slot-demo', 'crash-demo')
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO game_client_versions (game_id, version, bundle_url, status, published_at)
SELECT g.id, '0.1.0', CONCAT('/game/', g.game_code, '/'), 'published', UTC_TIMESTAMP(3)
FROM games g WHERE g.game_code IN ('slot-demo', 'crash-demo')
ON DUPLICATE KEY UPDATE status = VALUES(status);

SELECT '23-seed-extra-games-migration applied' AS note;
