-- 08-money-migration.sql
-- 将金额列统一为 BIGINT minor units (Scale=10000)
-- 依赖：06 pending_transactions, 04 wallet_pending_ops, 05 game_round_replay, 01 daily_settlements
-- 推荐顺序：make migrate-wallet migrate-replay migrate-orphan-trace && make migrate-money

USE fastgame;

-- pending_transactions
UPDATE pending_transactions
SET bet_amount = ROUND(bet_amount * 10000),
    win_amount = ROUND(win_amount * 10000)
WHERE bet_amount < 1000000 AND bet_amount > 0;

ALTER TABLE pending_transactions
  MODIFY bet_amount BIGINT NOT NULL COMMENT 'minor units, scale=10000',
  MODIFY win_amount BIGINT NOT NULL DEFAULT 0 COMMENT 'minor units';

-- wallet_pending_ops (from 04 or 01-init)
UPDATE wallet_pending_ops
SET bet_amount = ROUND(bet_amount * 10000),
    win_amount = ROUND(win_amount * 10000)
WHERE bet_amount < 1000000 AND bet_amount > 0;

ALTER TABLE wallet_pending_ops
  MODIFY bet_amount BIGINT NOT NULL DEFAULT 0 COMMENT 'minor units',
  MODIFY win_amount BIGINT NOT NULL DEFAULT 0 COMMENT 'minor units';

-- game_round_replay
UPDATE game_round_replay
SET bet_amount = ROUND(bet_amount * 10000)
WHERE bet_amount < 1000000 AND bet_amount > 0;

ALTER TABLE game_round_replay
  MODIFY bet_amount BIGINT NOT NULL COMMENT 'minor units';

-- daily_settlements
UPDATE daily_settlements
SET total_bet = ROUND(total_bet * 10000),
    total_win = ROUND(total_win * 10000)
WHERE total_bet < 1000000000 AND total_bet >= 0;

ALTER TABLE daily_settlements
  MODIFY total_bet BIGINT NOT NULL DEFAULT 0 COMMENT 'minor units',
  MODIFY total_win BIGINT NOT NULL DEFAULT 0 COMMENT 'minor units';

UPDATE game_configs
SET config_value = '{"min":10000,"max":10000000,"allowed":[10000,50000,100000,500000,1000000]}'
WHERE config_key = 'bet_limits';

SELECT '08-money-migration applied' AS note;
