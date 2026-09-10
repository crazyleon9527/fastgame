-- Admin TOTP recovery codes (hashed JSON array)
-- Requires 07-antiabuse-migration (totp_secret/totp_enabled); run migrate-antiabuse first if needed.
USE fastgame;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_secret'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_secret VARCHAR(64) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_enabled'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_enabled TINYINT NOT NULL DEFAULT 0',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'admin_users' AND COLUMN_NAME = 'totp_recovery_hashes'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE admin_users ADD COLUMN totp_recovery_hashes JSON NULL COMMENT ''bcrypt hashes of one-time recovery codes'' AFTER totp_enabled',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
