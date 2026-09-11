-- 21: Merchant game lobby view — single query for RGS launch list
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('21-merchant-lobby-view', 'v_merchant_game_lobby view for game launch API');

DROP VIEW IF EXISTS v_merchant_game_lobby;

CREATE VIEW v_merchant_game_lobby AS
SELECT
  m.id              AS merchant_id,
  m.merchant_code,
  m.name            AS merchant_name,
  mg.id             AS merchant_game_id,
  mg.sort_order     AS lobby_sort,
  mg.status         AS merchant_game_status,
  mg.min_bet_minor  AS merchant_min_bet_minor,
  mg.max_bet_minor  AS merchant_max_bet_minor,
  mg.rtp_tier_code  AS merchant_rtp_tier_code,
  g.id              AS game_id,
  g.game_code,
  g.name            AS game_name,
  g.game_type,
  g.default_rtp_ppm,
  g.min_bet_minor   AS game_min_bet_minor,
  g.max_bet_minor   AS game_max_bet_minor,
  g.client_version,
  g.thumbnail_url,
  g.status          AS game_status,
  gc.code           AS category_code,
  gc.name           AS category_name,
  COALESCE(mg.min_bet_minor, g.min_bet_minor) AS effective_min_bet_minor,
  COALESCE(mg.max_bet_minor, g.max_bet_minor) AS effective_max_bet_minor,
  rt.target_rtp_ppm AS tier_rtp_ppm
FROM merchants m
JOIN merchant_games mg ON mg.merchant_id = m.id AND mg.status = 1
JOIN games g ON g.id = mg.game_id AND g.status = 1
LEFT JOIN game_categories gc ON gc.id = g.category_id
LEFT JOIN game_rtp_tiers rt ON rt.game_id = g.id
  AND rt.tier_code = COALESCE(mg.rtp_tier_code, 'default')
  AND rt.status = 1;

SELECT '21-merchant-lobby-view-migration applied' AS note;
