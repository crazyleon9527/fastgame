-- ClickHouse 金额列统一为 Int64 minor units (Scale=10000)

-- 将历史 Decimal major 转为 minor
ALTER TABLE fastgame.game_round_settled
  UPDATE
    bet_amount = toInt64(toDecimal64(bet_amount, 4) * 10000),
    win_amount = toInt64(toDecimal64(win_amount, 4) * 10000),
    multiplier = toInt64(toDecimal64(multiplier, 4) * 10000),
    balance_after = toInt64(toDecimal64(balance_after, 4) * 10000)
  WHERE 1;

ALTER TABLE fastgame.game_event_bigwin
  UPDATE win_amount = toInt64(toDecimal64(win_amount, 4) * 10000)
  WHERE 1;

ALTER TABLE fastgame.game_wallet_rollback
  UPDATE amount = toInt64(toDecimal64(amount, 4) * 10000)
  WHERE 1;

ALTER TABLE fastgame.game_round_settled
  MODIFY COLUMN bet_amount Int64,
  MODIFY COLUMN win_amount Int64,
  MODIFY COLUMN multiplier Int64,
  MODIFY COLUMN balance_after Int64;

ALTER TABLE fastgame.game_event_bigwin
  MODIFY COLUMN win_amount Int64;

ALTER TABLE fastgame.game_wallet_rollback
  MODIFY COLUMN amount Int64;

DROP TABLE IF EXISTS fastgame.mv_rtp_hourly;

CREATE MATERIALIZED VIEW fastgame.mv_rtp_hourly
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (merchant_id, game_code, hour)
AS SELECT
    merchant_id,
    game_code,
    toStartOfHour(settled_at) AS hour,
    sum(bet_amount)           AS total_bet,
    sum(win_amount)           AS total_win,
    count()                   AS total_rounds
FROM fastgame.game_round_settled
GROUP BY merchant_id, game_code, hour;
