USE fastgame;

INSERT INTO merchants (merchant_code, name, private_key, allowed_ips, status)
VALUES ('m001', 'Demo Merchant', 'dev-secret-m001-change-in-prod', '["127.0.0.1","::1"]', 1)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  private_key = VALUES(private_key),
  allowed_ips = VALUES(allowed_ips);

INSERT INTO game_configs (merchant_id, game_code, config_key, config_value, rtp_tier, status)
SELECT m.id, 'fishing', 'rtp_tier', '{"tier":"high","target_rtp":0.96}', 'high', 1
FROM merchants m
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE
  config_value = VALUES(config_value),
  rtp_tier = VALUES(rtp_tier);

INSERT INTO game_configs (merchant_id, game_code, config_key, config_value, rtp_tier, status)
SELECT m.id, 'fishing', 'bet_limits', '{"min":10000,"max":10000000,"allowed":[10000,50000,100000,500000,1000000]}', NULL, 1
FROM merchants m
WHERE m.merchant_code = 'm001'
ON DUPLICATE KEY UPDATE
  config_value = VALUES(config_value);
