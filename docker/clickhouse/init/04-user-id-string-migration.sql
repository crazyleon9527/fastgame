-- ClickHouse user_id: UInt64 -> String（与 docs/database 及 B2B 下游玩家 ID 对齐）
SET mutations_sync = 2;

ALTER TABLE fastgame.game_round_settled
  UPDATE user_id = toString(user_id) WHERE 1;

ALTER TABLE fastgame.game_event_bigwin
  UPDATE user_id = toString(user_id) WHERE 1;

ALTER TABLE fastgame.game_wallet_rollback
  UPDATE user_id = toString(user_id) WHERE 1;

ALTER TABLE fastgame.game_round_settled
  MODIFY COLUMN user_id String;

ALTER TABLE fastgame.game_event_bigwin
  MODIFY COLUMN user_id String;

ALTER TABLE fastgame.game_wallet_rollback
  MODIFY COLUMN user_id String;
