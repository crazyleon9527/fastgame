-- 下游玩家 ID 统一为 VARCHAR(64)，与 docs/database 及 B2B 钱包对齐
USE fastgame;

ALTER TABLE pending_transactions
  MODIFY COLUMN user_id VARCHAR(64) NOT NULL;

ALTER TABLE wallet_pending_ops
  MODIFY COLUMN user_id VARCHAR(64) NOT NULL;

ALTER TABLE game_round_replay
  MODIFY COLUMN user_id VARCHAR(64) NOT NULL;
