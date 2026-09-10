-- 全链路 Trace 染色 — 沉淀至 ClickHouse 供客诉秒级定位

ALTER TABLE fastgame.game_round_settled
    ADD COLUMN IF NOT EXISTS trace_id String DEFAULT '' AFTER event_id;

ALTER TABLE fastgame.game_wallet_rollback
    ADD COLUMN IF NOT EXISTS trace_id String DEFAULT '' AFTER event_id;

CREATE TABLE IF NOT EXISTS fastgame.trace_spans
(
    trace_id     String,
    span_id      String,
    service      LowCardinality(String),
    operation    LowCardinality(String),
    round_id     String DEFAULT '',
    status       LowCardinality(String),
    detail       String DEFAULT '',
    duration_ms  UInt32 DEFAULT 0,
    occurred_at  DateTime64(3, 'UTC'),
    ingested_at  DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (trace_id, occurred_at, span_id)
TTL toDateTime(occurred_at) + INTERVAL 180 DAY
SETTINGS index_granularity = 8192;
