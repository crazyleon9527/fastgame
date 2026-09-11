-- 13: Settlement billing — weekly/monthly periods and line items linked to daily_settlements
USE fastgame;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('13-settlement-billing', 'Settlement periods and merchant settlement lines');

-- Billing period per merchant (week / month rollup)
CREATE TABLE IF NOT EXISTS settlement_periods (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id         BIGINT UNSIGNED NOT NULL,
  period_type         VARCHAR(16)     NOT NULL DEFAULT 'weekly' COMMENT 'daily/weekly/monthly',
  period_start        DATE            NOT NULL,
  period_end          DATE            NOT NULL,
  currency_code       CHAR(8)         NOT NULL DEFAULT 'USD',
  total_bet_minor     BIGINT          NOT NULL DEFAULT 0,
  total_win_minor     BIGINT          NOT NULL DEFAULT 0,
  ggr_minor           BIGINT          NOT NULL DEFAULT 0 COMMENT 'total_bet - total_win',
  commission_minor    BIGINT          NOT NULL DEFAULT 0 COMMENT 'platform share from GGR',
  net_payable_minor   BIGINT          NOT NULL DEFAULT 0 COMMENT 'amount due (platform receivable)',
  total_rounds        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status              VARCHAR(16)     NOT NULL DEFAULT 'draft' COMMENT 'draft/pending_review/confirmed/invoiced/paid',
  confirmed_by        BIGINT UNSIGNED NULL COMMENT 'admin_users.id',
  confirmed_at        DATETIME(3)     NULL,
  notes               VARCHAR(512)    NULL,
  created_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_period (merchant_id, period_type, period_start, period_end),
  KEY idx_merchant_status (merchant_id, status),
  KEY idx_period_range (period_start, period_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Merchant settlement billing period';

-- Line items: daily rollup, per-game breakdown, adjustments
CREATE TABLE IF NOT EXISTS merchant_settlement_lines (
  id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  settlement_period_id  BIGINT UNSIGNED NOT NULL,
  merchant_id           BIGINT UNSIGNED NOT NULL,
  line_type             VARCHAR(32)     NOT NULL DEFAULT 'daily_aggregate'
    COMMENT 'daily_aggregate/game_breakdown/commission/adjustment',
  ref_date              DATE            NULL COMMENT 'for daily_aggregate lines',
  game_id               BIGINT UNSIGNED NULL,
  daily_settlement_id   BIGINT UNSIGNED NULL COMMENT 'link to daily_settlements.id',
  description           VARCHAR(256)    NULL,
  total_bet_minor       BIGINT          NOT NULL DEFAULT 0,
  total_win_minor       BIGINT          NOT NULL DEFAULT 0,
  total_rounds          BIGINT UNSIGNED NOT NULL DEFAULT 0,
  commission_rate_ppm   BIGINT          NULL COMMENT 'snapshot of rate at close',
  commission_minor      BIGINT          NOT NULL DEFAULT 0,
  amount_minor          BIGINT          NOT NULL DEFAULT 0 COMMENT 'signed line amount',
  sort_order            INT             NOT NULL DEFAULT 0,
  created_at            DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_period_sort (settlement_period_id, sort_order),
  KEY idx_merchant_date (merchant_id, ref_date),
  KEY idx_daily_settlement (daily_settlement_id),
  KEY idx_game_id (game_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Settlement period line items';

-- Link daily_settlements to a billing period once rolled up
SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'daily_settlements' AND COLUMN_NAME = 'settlement_period_id'
);
SET @sql = IF(@col_exists = 0,
  'ALTER TABLE daily_settlements ADD COLUMN settlement_period_id BIGINT UNSIGNED NULL COMMENT ''FK to settlement_periods when rolled up'' AFTER status',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @idx_exists = (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = 'fastgame' AND TABLE_NAME = 'daily_settlements' AND INDEX_NAME = 'idx_settlement_period'
);
SET @sql = IF(@idx_exists = 0,
  'CREATE INDEX idx_settlement_period ON daily_settlements (settlement_period_id)',
  'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SELECT '13-settlement-billing-migration applied' AS note;
