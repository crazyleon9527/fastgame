-- 20: Archive / retention policy registry for high-volume audit tables
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('20-audit-archive-policy', 'Archive retention policy for audit_logs and game_sessions');

CREATE TABLE IF NOT EXISTS schema_archive_policies (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  table_name        VARCHAR(64)     NOT NULL,
  hot_retention_days INT UNSIGNED   NOT NULL DEFAULT 90 COMMENT 'keep in MySQL hot tier',
  cold_retention_days INT UNSIGNED  NULL COMMENT 'optional cold storage before purge',
  archive_strategy  VARCHAR(32)     NOT NULL DEFAULT 'delete' COMMENT 'delete/export_to_s3/partition_drop',
  partition_column  VARCHAR(64)     NULL COMMENT 'e.g. created_at for monthly partitions',
  last_archived_at  DATETIME(3)     NULL,
  notes             VARCHAR(512)    NULL,
  status            TINYINT         NOT NULL DEFAULT 1,
  created_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_table_name (table_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Table archival retention policies';

INSERT INTO schema_archive_policies (table_name, hot_retention_days, cold_retention_days, archive_strategy, partition_column, notes)
VALUES
  ('audit_logs', 180, 730, 'export_to_s3', 'created_at', 'Admin audit; export monthly to object storage then purge'),
  ('game_sessions', 90, 365, 'delete', 'created_at', 'Session registry; Redis is hot path, MySQL for reconciliation only'),
  ('risk_alerts', 365, NULL, 'delete', 'created_at', 'Keep 1 year for compliance review')
ON DUPLICATE KEY UPDATE hot_retention_days = VALUES(hot_retention_days);

-- Purge helper index: game_sessions by created_at for batch archive jobs
SET @idx_exists = (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='fastgame' AND TABLE_NAME='game_sessions' AND INDEX_NAME='idx_created_status');
SET @sql = IF(@idx_exists=0, 'CREATE INDEX idx_created_status ON game_sessions (created_at, status)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SELECT '20-audit-archive-policy-migration applied' AS note;
