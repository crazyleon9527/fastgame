-- 04-user-id-string-migration.sql
-- ClickHouse user_id: UInt64 -> String（与 MySQL / B2B 上游的字符串玩家 ID 对齐）
--
-- 【为什么必须重建表，而不能 ALTER】
--   1) user_id 是 ORDER BY 的 key 列，ClickHouse 禁止 UPDATE key column
--      → 原方案的 `UPDATE user_id = toString(user_id)` 报 CANNOT_UPDATE_COLUMN
--   2) 同样因为它属于 key，`MODIFY COLUMN ... String` 报 ALTER_OF_COLUMN_IS_FORBIDDEN
--      （"can change the representation of primary key"）
--   因此唯一可行路径是「按新类型重建表」。本次迁移时三张表均为 0 行，
--   重建无数据损失；ORDER BY 保持不变，按 user_id 查询的性能特征不变。
--
-- 【执行方式】
--   make migrate-ch-user-id
--   本文件不会被 clickhouse-init 自动执行（见 docker/clickhouse/init-db.sh 说明），
--   属存量库增量迁移，需显式调用。

-- 1) 先删依赖 game_round_settled 的物化视图，否则无法重建基表
DROP VIEW IF EXISTS fastgame.mv_rtp_hourly;

-- 2) 重建三张表，仅 user_id 类型改为 String
DROP TABLE IF EXISTS fastgame.game_round_settled;
CREATE TABLE fastgame.game_round_settled
(
    event_id       UUID,
    trace_id       String DEFAULT '',
    round_id       String,
    user_id        String,
    merchant_id    UInt64,
    game_code      LowCardinality(String),
    bet_amount     Int64 COMMENT 'minor units scale=10000',
    win_amount     Int64,
    multiplier     Int64 COMMENT 'multiplier minor e.g. 15000=1.5x',
    rtp_tier       LowCardinality(String),
    balance_after  Int64,
    settled_at     DateTime64(3, 'UTC'),
    ingested_at    DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(settled_at)
ORDER BY (merchant_id, user_id, settled_at, round_id)
TTL toDateTime(settled_at) + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

DROP TABLE IF EXISTS fastgame.game_event_bigwin;
CREATE TABLE fastgame.game_event_bigwin
(
    event_id     UUID,
    round_id     String,
    user_id      String,
    merchant_id  UInt64,
    game_code    LowCardinality(String),
    win_amount   Int64,
    multiplier   Int64,
    occurred_at  DateTime64(3, 'UTC'),
    ingested_at  DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (occurred_at, merchant_id, user_id)
SETTINGS index_granularity = 8192;

DROP TABLE IF EXISTS fastgame.game_wallet_rollback;
CREATE TABLE fastgame.game_wallet_rollback
(
    event_id      UUID,
    trace_id      String DEFAULT '',
    round_id      String,
    user_id       String,
    merchant_id   UInt64,
    rollback_type LowCardinality(String),
    amount        Int64,
    reason        String,
    status        LowCardinality(String) DEFAULT 'pending',
    occurred_at   DateTime64(3, 'UTC'),
    ingested_at   DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (status, occurred_at, round_id)
SETTINGS index_granularity = 8192;

-- 3) 重建依赖 game_round_settled 的 RTP 小时聚合视图（列与顺序保持原样）
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
