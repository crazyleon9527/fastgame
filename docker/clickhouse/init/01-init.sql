-- FastGame ClickHouse 初始化脚本
-- 海量注单明细、RTP 偏差统计、玩家行为分析

CREATE DATABASE IF NOT EXISTS fastgame;

-- 核心注单明细表 (对应 Kafka topic: game.round.settled)
CREATE TABLE IF NOT EXISTS fastgame.game_round_settled
(
    event_id       UUID,
    round_id       String,
    user_id        UInt64,
    merchant_id    UInt64,
    game_code      LowCardinality(String),
    bet_amount     Decimal(18, 4),
    win_amount     Decimal(18, 4),
    multiplier     Decimal(10, 4),
    rtp_tier       LowCardinality(String),
    balance_after  Decimal(18, 4),
    settled_at     DateTime64(3, 'UTC'),
    ingested_at    DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(settled_at)
ORDER BY (merchant_id, user_id, settled_at, round_id)
TTL toDateTime(settled_at) + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- 大奖事件表 (对应 Kafka topic: game.event.bigwin)
CREATE TABLE IF NOT EXISTS fastgame.game_event_bigwin
(
    event_id     UUID,
    round_id     String,
    user_id      UInt64,
    merchant_id  UInt64,
    game_code    LowCardinality(String),
    win_amount   Decimal(18, 4),
    multiplier   Decimal(10, 4),
    occurred_at  DateTime64(3, 'UTC'),
    ingested_at  DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (occurred_at, merchant_id, user_id)
SETTINGS index_granularity = 8192;

-- 钱包冲正事件表 (对应 Kafka topic: game.wallet.rollback)
CREATE TABLE IF NOT EXISTS fastgame.game_wallet_rollback
(
    event_id      UUID,
    round_id      String,
    user_id       UInt64,
    merchant_id   UInt64,
    rollback_type LowCardinality(String),
    amount        Decimal(18, 4),
    reason        String,
    status        LowCardinality(String) DEFAULT 'pending',
    occurred_at   DateTime64(3, 'UTC'),
    ingested_at   DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (status, occurred_at, round_id)
SETTINGS index_granularity = 8192;

-- RTP 实时偏差聚合视图 (供 Sponge Admin 报表只读查询)
CREATE MATERIALIZED VIEW IF NOT EXISTS fastgame.mv_rtp_hourly
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
