-- 05-dedup-migration.sql
-- ClickHouse 结算表改用 ReplacingMergeTree 去重 + event_id 类型修正
--
-- 【为什么改】
--   1) 原引擎是 MergeTree，没有去重能力。outbox 的投递保证是 at-least-once，
--      消费端幂等只是应用层的一层；一旦幂等失效（Redis/DB 被清、事务回滚、
--      旧格式消息没有 eventId），重复行会直接落库，RTP 统计与账单随之翻倍。
--      改用 ReplacingMergeTree，让存储层按业务主键自身折叠重复，形成结构性兜底。
--      （查询时如需立即去重，可加 FINAL，或依赖后台 merge。）
--   2) event_id 原为 UUID 类型，空值写入直接报 CANNOT_PARSE_UUID
--      （实测：对 event_id 做 = '' 比较即报该错）。事件 ID 缺失会导致整批插入
--      失败、事件丢失。改为 String。
--   3) 排序键补 round_id 作为最末一级：让"同一局被重复投递"的行落在同一去重键上
--      从而折叠。前缀仍是 merchant_id/user_id/settled_at，按时间与玩家的查询
--      及分区裁剪不受影响。
--
-- 【设计取舍：用 staging 表而不是 RENAME 覆盖】
--   本脚本可重复执行。做法是先把新结构建成 `<表>_new`，把数据拷过去，
--   确认成功后再把旧表改名为 `<表>_legacy_backup`、把 _new 改名为正式名。
--   这样任何一步失败都不会让正式表消失。
--   （曾经的写法是先把正式表 RENAME 成备份、再建新表——一旦脚本被截断，
--     正式表就没了，属于危险写法。）
--
-- 【执行方式】不被 clickhouse-init 自动执行（见 docker/clickhouse/init-db.sh），
--   属存量库增量迁移，需显式执行：
--     .\scripts\apply_clickhouse_migration.ps1 docker\clickhouse\init\05-dedup-migration.sql
--   或 make migrate-ch-dedup

-- ============================================================ 0. 清理上次残留
DROP TABLE IF EXISTS fastgame.game_round_settled_new;
DROP TABLE IF EXISTS fastgame.game_event_bigwin_new;
DROP TABLE IF EXISTS fastgame.game_wallet_rollback_new;

-- ============================================================ 1. 建 staging 新表
CREATE TABLE fastgame.game_round_settled_new
(
    event_id       String,
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
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(settled_at)
ORDER BY (merchant_id, user_id, settled_at, round_id)
TTL toDateTime(settled_at) + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE fastgame.game_event_bigwin_new
(
    event_id     String,
    round_id     String,
    user_id      String,
    merchant_id  UInt64,
    game_code    LowCardinality(String),
    win_amount   Int64,
    multiplier   Int64,
    occurred_at  DateTime64(3, 'UTC'),
    ingested_at  DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (occurred_at, merchant_id, user_id, round_id)
SETTINGS index_granularity = 8192;

CREATE TABLE fastgame.game_wallet_rollback_new
(
    event_id      String,
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
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (round_id, occurred_at, status)
SETTINGS index_granularity = 8192;

-- ============================================================ 2. 拷贝历史数据
-- 若某个正式表已是新结构（重复执行本迁移），它同样满足下面的 SELECT 列清单，
-- 因此拷贝依然成立；已迁移过的库此处是"再拷一份到 staging"，随后替换为等价表。
-- 物化视图此时尚未重建，所以这些 INSERT 不会污染 mv_rtp_hourly。
INSERT INTO fastgame.game_round_settled_new
    (event_id, trace_id, round_id, user_id, merchant_id, game_code,
     bet_amount, win_amount, multiplier, rtp_tier, balance_after, settled_at, ingested_at)
SELECT
    toString(event_id), trace_id, round_id, user_id, merchant_id, game_code,
    bet_amount, win_amount, multiplier, rtp_tier, balance_after, settled_at, ingested_at
FROM fastgame.game_round_settled;

INSERT INTO fastgame.game_event_bigwin_new
    (event_id, round_id, user_id, merchant_id, game_code, win_amount, multiplier, occurred_at, ingested_at)
SELECT
    toString(event_id), round_id, user_id, merchant_id, game_code, win_amount, multiplier, occurred_at, ingested_at
FROM fastgame.game_event_bigwin;

INSERT INTO fastgame.game_wallet_rollback_new
    (event_id, trace_id, round_id, user_id, merchant_id, rollback_type, amount, reason, status, occurred_at, ingested_at)
SELECT
    toString(event_id), trace_id, round_id, user_id, merchant_id, rollback_type, amount, reason, status, occurred_at, ingested_at
FROM fastgame.game_wallet_rollback;

-- ============================================================ 3. 原子改名替换
-- 先删视图（它依赖 game_round_settled），再替换基表，最后在步骤 4 重建。
DROP VIEW IF EXISTS fastgame.mv_rtp_hourly;

DROP TABLE IF EXISTS fastgame.game_round_settled_legacy_backup;
DROP TABLE IF EXISTS fastgame.game_event_bigwin_legacy_backup;
DROP TABLE IF EXISTS fastgame.game_wallet_rollback_legacy_backup;

RENAME TABLE fastgame.game_round_settled   TO fastgame.game_round_settled_legacy_backup,
             fastgame.game_round_settled_new TO fastgame.game_round_settled;

RENAME TABLE fastgame.game_event_bigwin    TO fastgame.game_event_bigwin_legacy_backup,
             fastgame.game_event_bigwin_new  TO fastgame.game_event_bigwin;

RENAME TABLE fastgame.game_wallet_rollback    TO fastgame.game_wallet_rollback_legacy_backup,
             fastgame.game_wallet_rollback_new TO fastgame.game_wallet_rollback;

-- ============================================================ 4. 重建聚合视图
-- 放在数据拷贝之后：视图只捕获"重建之后"的 INSERT，历史数据已在正式表里。
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

-- 显式回填一次历史聚合，保证报表在迁移后立即可用
INSERT INTO fastgame.mv_rtp_hourly
SELECT
    merchant_id,
    game_code,
    toStartOfHour(settled_at) AS hour,
    sum(bet_amount)           AS total_bet,
    sum(win_amount)           AS total_win,
    count()                   AS total_rounds
FROM fastgame.game_round_settled
GROUP BY merchant_id, game_code, hour;
