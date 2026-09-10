-- 确定性回放：仅持久化种子 + 基础输入，场景由 PRNG 按需复现
CREATE TABLE IF NOT EXISTS game_round_replay (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  round_id      VARCHAR(64)     NOT NULL COMMENT 'Nonce / Round ID',
  merchant_code VARCHAR(32)     NOT NULL,
  user_id       BIGINT UNSIGNED NOT NULL,
  game_code     VARCHAR(32)     NOT NULL,
  server_seed   VARCHAR(128)    NOT NULL,
  client_seed   VARCHAR(128)    NOT NULL,
  nonce         VARCHAR(64)     NOT NULL,
  bet_amount    DECIMAL(18,4)   NOT NULL,
  sequence_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at    TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_round_id (round_id),
  KEY idx_user_created (user_id, created_at),
  KEY idx_merchant_created (merchant_code, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Deterministic replay inputs';
