-- 19: Add merchant_id FK-style columns where merchant_code string was used
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('19-normalize-merchant-refs', 'Add merchant_id to legacy merchant_code tables');

-- risk_alerts
SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='risk_alerts' AND COLUMN_NAME='merchant_id');
SET @sql = IF(@col_exists=0, 'ALTER TABLE risk_alerts ADD COLUMN merchant_id BIGINT UNSIGNED NULL AFTER id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE risk_alerts ra
JOIN merchants m ON m.merchant_code = ra.merchant_code
SET ra.merchant_id = m.id
WHERE ra.merchant_id IS NULL AND ra.merchant_code != '';

SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='risk_alerts' AND INDEX_NAME='idx_merchant_id_created');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_id_created ON risk_alerts (merchant_id, created_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- wallet_pending_ops
SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='wallet_pending_ops' AND COLUMN_NAME='merchant_id');
SET @sql = IF(@col_exists=0, 'ALTER TABLE wallet_pending_ops ADD COLUMN merchant_id BIGINT UNSIGNED NULL AFTER round_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE wallet_pending_ops w
JOIN merchants m ON m.merchant_code = w.merchant_code
SET w.merchant_id = m.id
WHERE w.merchant_id IS NULL;

SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='wallet_pending_ops' AND INDEX_NAME='idx_merchant_id_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_id_status ON wallet_pending_ops (merchant_id, status, updated_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- pending_transactions
SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='pending_transactions' AND COLUMN_NAME='merchant_id');
SET @sql = IF(@col_exists=0, 'ALTER TABLE pending_transactions ADD COLUMN merchant_id BIGINT UNSIGNED NULL AFTER round_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE pending_transactions p
JOIN merchants m ON m.merchant_code = p.merchant_code
SET p.merchant_id = m.id
WHERE p.merchant_id IS NULL;

SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='pending_transactions' AND INDEX_NAME='idx_merchant_id_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_id_status ON pending_transactions (merchant_id, status, updated_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- game_round_replay
SET @col_exists = (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_round_replay' AND COLUMN_NAME='merchant_id');
SET @sql = IF(@col_exists=0, 'ALTER TABLE game_round_replay ADD COLUMN merchant_id BIGINT UNSIGNED NULL AFTER round_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE game_round_replay r
JOIN merchants m ON m.merchant_code = r.merchant_code
SET r.merchant_id = m.id
WHERE r.merchant_id IS NULL;

SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_round_replay' AND INDEX_NAME='idx_merchant_id_created');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_id_created ON game_round_replay (merchant_id, created_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT '19-normalize-merchant-refs-migration applied' AS note;
