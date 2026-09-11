-- 17: Index optimization batch 1 — hot query paths
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('17-index-optimize', 'Index optimization batch 1');

-- merchants: filter active merchants
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='merchants' AND INDEX_NAME='idx_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_status ON merchants (status)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- daily_settlements: admin list by merchant + date + status
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='daily_settlements' AND INDEX_NAME='idx_merchant_date_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_date_status ON daily_settlements (merchant_id, settle_date, status)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- commission_rules: lookup active rule for merchant+game
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='commission_rules' AND INDEX_NAME='idx_merchant_game_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_game_status ON commission_rules (merchant_id, game_id, status, effective_from)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- game_configs: list by merchant without game filter
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_configs' AND INDEX_NAME='idx_merchant_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_status ON game_configs (merchant_id, status)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- pending_transactions: reconciler scan
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='pending_transactions' AND INDEX_NAME='idx_status_updated');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_status_updated ON pending_transactions (status, updated_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- risk_alerts: filter by merchant
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='risk_alerts' AND INDEX_NAME='idx_merchant_created');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_created ON risk_alerts (merchant_code, created_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- game_round_replay: lookup by game
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_round_replay' AND INDEX_NAME='idx_game_created');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_game_created ON game_round_replay (game_code, created_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- settlement_periods: open periods by status
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='settlement_periods' AND INDEX_NAME='idx_status_period');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_status_period ON settlement_periods (status, period_end)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT '17-index-optimize-migration applied' AS note;
