-- 22: CHECK constraints on status enums (MySQL 8.0.16+)
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('22-status-checks', 'CHECK constraints on status columns');

-- Helper: add CHECK only if not already present (MySQL has no IF NOT EXISTS for constraints)
-- settlement_periods.status
SET @chk_exists = (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='settlement_periods' AND CONSTRAINT_NAME='chk_settlement_period_status');
SET @sql = IF(@chk_exists=0,
  'ALTER TABLE settlement_periods ADD CONSTRAINT chk_settlement_period_status CHECK (status IN (''draft'',''pending_review'',''confirmed'',''invoiced'',''paid'',''cancelled''))',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- games.status: 0=off 1=live 2=maintenance
SET @chk_exists = (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='games' AND CONSTRAINT_NAME='chk_games_status');
SET @sql = IF(@chk_exists=0,
  'ALTER TABLE games ADD CONSTRAINT chk_games_status CHECK (status IN (0, 1, 2))',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- merchant_games.status
SET @chk_exists = (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='merchant_games' AND CONSTRAINT_NAME='chk_merchant_games_status');
SET @sql = IF(@chk_exists=0,
  'ALTER TABLE merchant_games ADD CONSTRAINT chk_merchant_games_status CHECK (status IN (0, 1))',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- game_client_versions.status
SET @chk_exists = (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_client_versions' AND CONSTRAINT_NAME='chk_client_version_status');
SET @sql = IF(@chk_exists=0,
  'ALTER TABLE game_client_versions ADD CONSTRAINT chk_client_version_status CHECK (status IN (''draft'',''published'',''deprecated''))',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- player_merchant_profiles.status
SET @chk_exists = (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='player_merchant_profiles' AND CONSTRAINT_NAME='chk_player_profile_status');
SET @sql = IF(@chk_exists=0,
  'ALTER TABLE player_merchant_profiles ADD CONSTRAINT chk_player_profile_status CHECK (status IN (0, 1))',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT '22-status-checks-migration applied' AS note;
