-- =============================================================================
-- 模块三：报表与商业智能中心 (Reporting & Analytics Domain)
-- 适用引擎: ClickHouse 22+
-- =============================================================================

CREATE DATABASE IF NOT EXISTS rgs_analytics;

-- -----------------------------------------------------------------------------
-- 1. 商户日终综合营运报表 (rpt_daily_merchant_summary) - 财务对账核心
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS rgs_analytics.rpt_daily_merchant_summary;
CREATE TABLE rgs_analytics.rpt_daily_merchant_summary (
                                                          `stat_date` Date COMMENT '统计日期 (UTC)',
                                                          `merchant_code` LowCardinality(String) COMMENT '商户唯一代号',
                                                          `currency` LowCardinality(String) COMMENT '结算币种 (USD, BRL, PHP 等)',
                                                          `is_demo` UInt8 COMMENT '模式: 0-真金报表, 1-试玩报表',

    -- 核心财务度量 (分)
                                                          `total_rounds` UInt64 COMMENT '总下注/抛竿局数',
                                                          `total_bet` Int64 COMMENT '总有效投注额 (分)',
                                                          `total_win` Int64 COMMENT '总有效派彩额 (分)',
                                                          `ggr` Int64 COMMENT '平台毛利 (分): total_bet - total_win',
                                                          `overall_rtp` Float32 COMMENT '全日综合实际 RTP: (total_win / total_bet) * 100',

    -- 客流与频次
                                                          `active_users` UInt32 COMMENT '当日独立活跃玩家数 (DAU)',
                                                          `new_users` UInt32 COMMENT '当日首次在该商户体验我方游戏的全新玩家数',
                                                          `avg_bet_per_user` Int64 COMMENT '人均投注额 (分): total_bet / active_users',
                                                          `avg_rounds_per_user` UInt32 COMMENT '人均局数: total_rounds / active_users',

                                                          `updated_at` DateTime DEFAULT now() COMMENT '报表生成/重算刷新时间'
)
    ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(stat_date)
PRIMARY KEY (merchant_code, stat_date, currency, is_demo)
ORDER BY (merchant_code, stat_date, currency, is_demo)
SETTINGS index_granularity = 8192;

-- -----------------------------------------------------------------------------
-- 2. 游戏机台日表现透视表 (rpt_daily_game_performance) - 选品与数学监控
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS rgs_analytics.rpt_daily_game_performance;
CREATE TABLE rgs_analytics.rpt_daily_game_performance (
                                                          `stat_date` Date COMMENT '统计日期 (UTC)',
                                                          `merchant_code` LowCardinality(String) COMMENT '商户代号',
                                                          `game_code` LowCardinality(String) COMMENT '游戏代号 (fishing_deep_sea 等)',
                                                          `game_category` LowCardinality(String) COMMENT '品类 (ARCADE, SLOT, MINES 等)',
                                                          `currency` LowCardinality(String) COMMENT '币种',
                                                          `is_demo` UInt8 COMMENT '0-真金, 1-试玩',

    -- 游戏维度数据
                                                          `round_count` UInt64 COMMENT '该游戏当天总运转局数',
                                                          `player_count` UInt32 COMMENT '该游戏当天独立玩家数',
                                                          `total_bet` Int64 COMMENT '总投注额 (分)',
                                                          `total_win` Int64 COMMENT '总派彩额 (分)',
                                                          `ggr` Int64 COMMENT '该款游戏为商户贡献的毛利 (分)',

    -- 数学审计指标
                                                          `actual_rtp` Float32 COMMENT '该机台全天实际产生 RTP (%)',
                                                          `config_rtp` Float32 COMMENT '商户后台为该机台配置的理论 RTP (%)',
                                                          `rtp_deviation` Float32 COMMENT '实际与理论的偏差值 (%): actual_rtp - config_rtp',
                                                          `max_multiplier_hit` Float32 COMMENT '该游戏当天爆出的全天单局最大倍数',

                                                          `updated_at` DateTime DEFAULT now() COMMENT '最后计算时间'
)
    ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(stat_date)
PRIMARY KEY (merchant_code, game_code, stat_date, currency, is_demo)
ORDER BY (merchant_code, game_code, stat_date, currency, is_demo)
SETTINGS index_granularity = 8192;

-- -----------------------------------------------------------------------------
-- 3. 24小时分时走势表 (rpt_hourly_traffic_monitor) - 运营客流峰值与健康度
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS rgs_analytics.rpt_hourly_traffic_monitor;
CREATE TABLE rgs_analytics.rpt_hourly_traffic_monitor (
                                                          `stat_hour` DateTime COMMENT '统计整点 (如 2026-09-14 14:00:00 UTC)',
                                                          `merchant_code` LowCardinality(String) COMMENT '商户代号',
                                                          `game_code` LowCardinality(String) COMMENT '游戏代号 (或 ALL 代表全平台汇总)',
                                                          `currency` LowCardinality(String) COMMENT '币种',

                                                          `round_count` UInt32 COMMENT '该整点内总有效局数 (衡量并发压力)',
                                                          `total_bet` Int64 COMMENT '该整点总投注 (分)',
                                                          `total_win` Int64 COMMENT '该整点总派彩 (分)',
                                                          `ggr` Int64 COMMENT '该整点毛利 (分)',
                                                          `avg_latency_ms` UInt16 COMMENT '该小时内接口平均处理网络延迟 (微服务健康监控)',

                                                          `updated_at` DateTime DEFAULT now() COMMENT '更新时间'
)
    ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(stat_hour)
PRIMARY KEY (merchant_code, stat_hour, game_code, currency)
ORDER BY (merchant_code, stat_hour, game_code, currency)
SETTINGS index_granularity = 8192;

-- -----------------------------------------------------------------------------
-- 4. 玩家大户与风险排查表 (rpt_daily_player_summary) - VIP与防刷监控
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS rgs_analytics.rpt_daily_player_summary;
CREATE TABLE rgs_analytics.rpt_daily_player_summary (
                                                        `stat_date` Date COMMENT '统计日期',
                                                        `merchant_code` LowCardinality(String) COMMENT '商户代号',
                                                        `user_id` String COMMENT '下游商户侧玩家唯一ID',
                                                        `currency` LowCardinality(String) COMMENT '结算币种',

                                                        `rounds_played` UInt32 COMMENT '该玩家全天总抛竿/投注次数',
                                                        `favorite_game` LowCardinality(String) COMMENT '该玩家下注最多的偏好游戏代号',
                                                        `total_bet` Int64 COMMENT '该玩家全天累计投注 (分)',
                                                        `total_win` Int64 COMMENT '该玩家全天累计派彩 (分)',
                                                        `net_profit` Int64 COMMENT '玩家个人全天净利润 (分): total_win - total_bet (>0 代表玩家大赚赢钱)',
                                                        `max_single_win` Int64 COMMENT '该玩家单局最高中奖额 (分)',
                                                        `max_single_mult` Float32 COMMENT '该玩家单局最高爆奖倍数',

                                                        `updated_at` DateTime DEFAULT now() COMMENT '更新时间'
)
    ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(stat_date)
PRIMARY KEY (merchant_code, stat_date, user_id, currency)
ORDER BY (merchant_code, stat_date, user_id, currency)
SETTINGS index_granularity = 8192;

-- -----------------------------------------------------------------------------
-- 5. 全服超大爆奖审计与战报事实表 (rpt_big_win_records)
-- 触发条件: 派彩倍数 >= 50x 或绝对派彩金额 >= $1,000 (分)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS rgs_analytics.rpt_big_win_records;
CREATE TABLE rgs_analytics.rpt_big_win_records (
                                                   `round_id` String COMMENT '单局流水号 (唯一关联，可直接点击调取回放)',
                                                   `stat_date` Date COMMENT '产生日期 (用于分区)',
                                                   `merchant_code` LowCardinality(String) COMMENT '商户代号',
                                                   `game_code` LowCardinality(String) COMMENT '中奖游戏代号',
                                                   `user_id` String COMMENT '中奖玩家ID',
                                                   `currency` LowCardinality(String) COMMENT '币种',

                                                   `bet_amount` Int64 COMMENT '触发该大奖时的投注本金 (分)',
                                                   `win_amount` Int64 COMMENT '实际爆出的派彩金额 (分)',
                                                   `multiplier` Float32 COMMENT '实际爆出的震撼倍数 (如 250.0x, 1000.0x)',
                                                   `win_tier` LowCardinality(String) COMMENT '大奖评级: BIG_WIN (50-100x), MEGA_WIN (100-500x), EPIC_WIN (500x+)',

                                                   `server_seed_hash` String COMMENT '种子哈希快照 (用于风控取证防作弊)',
                                                   `created_at` DateTime COMMENT '大奖产生的绝对时间 (UTC)'
)
    ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(stat_date)
PRIMARY KEY (merchant_code, stat_date, round_id)
ORDER BY (merchant_code, stat_date, round_id)
SETTINGS index_granularity = 8192;


-- =============================================================================
-- MySQL 8.0: 报表导出异步任务表
-- =============================================================================

DROP TABLE IF EXISTS `rpt_export_tasks`;
CREATE TABLE `rpt_export_tasks` (
                                    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '任务自增主键ID',
                                    `task_id` VARCHAR(64) NOT NULL COMMENT '任务对外唯一代号 (UUID)',
                                    `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID',
                                    `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户代号',
                                    `operator_id` BIGINT UNSIGNED NOT NULL COMMENT '发起导出的商户操作人员用户ID',
                                    `report_type` VARCHAR(32) NOT NULL COMMENT '导出的报表类型: ROUNDS_DETAIL (原始注单), DAILY_SUMMARY (日总账), PLAYER_RANK (玩家排行)',
                                    `filter_params` JSON NOT NULL COMMENT '导出时的查询筛选条件快照 (日期范围、币种、游戏等)',
                                    `file_format` VARCHAR(8) NOT NULL DEFAULT 'CSV' COMMENT '导出格式: CSV, XLSX',
                                    `file_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '生成的文件大小 (字节)',
                                    `download_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '生成的临时安全下载地址 (S3/OSS 预签名 URL)',
                                    `task_status` VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT '任务状态: PENDING (排队中), PROCESSING (生成中), SUCCESS (已就绪), FAILED (生成失败)',
                                    `error_msg` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '失败原因说明',
                                    `expire_at` DATETIME NOT NULL COMMENT '文件下载链接失效时间 (通常保留 72 小时自动清理)',
                                    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '提交时间',
                                    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                    PRIMARY KEY (`id`),
                                    UNIQUE KEY `uk_task_id` (`task_id`),
                                    KEY `idx_merchant_status` (`merchant_id`, `task_status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户后台大数据报表异步导出与下载任务表';