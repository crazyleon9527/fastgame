-- 孤儿注单补偿表 + 全链路 Trace 关联
CREATE TABLE IF NOT EXISTS pending_transactions (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  trace_id         VARCHAR(64)     NOT NULL COMMENT '全链路 TraceID',
  round_id         VARCHAR(64)     NOT NULL COMMENT 'Round / Nonce',
  merchant_code    VARCHAR(32)     NOT NULL,
  user_id          BIGINT UNSIGNED NOT NULL,
  game_code        VARCHAR(32)     NOT NULL,
  phase            VARCHAR(32)     NOT NULL COMMENT 'bet_debited|win_pending|settled|orphan',
  status           VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pending|done|failed',
  bet_amount       DECIMAL(18,4)   NOT NULL,
  win_amount       DECIMAL(18,4)   NOT NULL DEFAULT 0,
  expected_action  VARCHAR(16)     NOT NULL COMMENT 'settle_win|rollback_bet|none',
  wallet_bet_status VARCHAR(16)    NOT NULL DEFAULT 'confirmed',
  wallet_win_status VARCHAR(16)     NOT NULL DEFAULT 'unknown',
  retry_count      INT             NOT NULL DEFAULT 0,
  last_error       VARCHAR(512)    NULL,
  created_at       TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_round_id (round_id),
  KEY idx_status_created (status, created_at),
  KEY idx_trace_id (trace_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='孤儿注单自动对账补偿';
