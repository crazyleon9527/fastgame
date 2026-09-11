-- 18: Index optimization batch 2 — composite keys and column tweaks
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('18-index-optimize-2', 'Index optimization batch 2');

-- audit_logs: time-range purge / archive
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='audit_logs' AND INDEX_NAME='idx_created_action');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_created_action ON audit_logs (created_at, action)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- player_merchant_profiles: active players recently seen
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='player_merchant_profiles' AND INDEX_NAME='idx_status_last_played');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_status_last_played ON player_merchant_profiles (status, last_played_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- merchant_webhooks: dispatch queue scan
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='merchant_webhooks' AND INDEX_NAME='idx_status_event');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_status_event ON merchant_webhooks (status, event_type)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- game_client_versions: latest published per game
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_client_versions' AND INDEX_NAME='idx_game_published');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_game_published ON game_client_versions (game_id, status, published_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- wallet_pending_ops: merchant scoped pending scan
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='wallet_pending_ops' AND INDEX_NAME='idx_merchant_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_merchant_status ON wallet_pending_ops (merchant_code, status, updated_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- games: lobby listing by type + status
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='games' AND INDEX_NAME='idx_type_status_launched');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_type_status_launched ON games (game_type, status, launched_at)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT '18-index-optimize-migration applied' AS note;
