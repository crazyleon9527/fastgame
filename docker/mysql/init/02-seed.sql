USE fastgame;

INSERT INTO merchants (merchant_code, name, private_key, status)
VALUES ('m001', 'Demo Merchant', 'dev-secret-m001-change-in-prod', 1)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  private_key = VALUES(private_key);

INSERT INTO game_configs (merchant_id, game_code, config_key, config_value, rtp_tier, status)
SELECT m.id, 'fishing', 'rtp_tier', '{"tier":"high","target_rtp":0.96}', 'high', 1
FROM merchants m
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE
  config_value = VALUES(config_value),
  rtp_tier = VALUES(rtp_tier);

INSERT INTO game_configs (merchant_id, game_code, config_key, config_value, rtp_tier, status)
SELECT m.id, 'fishing', 'bet_limits', '{"min":1,"max":1000,"allowed":[1,5,10,50,100]}', NULL, 1
FROM merchants m
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE
  config_value = VALUES(config_value);
