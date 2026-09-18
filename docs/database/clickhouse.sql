-- =============================================================================
-- ClickHouse 引擎：全兼容海量游戏结算注单事实表
-- 局完成后，Go-Zero 管道异步投递一条 Kafka 消息，由消费组批量（如每 1,000 条或每 1 秒）批刷入 ClickHouse。这里完整记录确定性算法证据链和前端演播 JSON 回放包
-- 适用引擎: ClickHouse 22+
-- =============================================================================

-- =============================================================================
-- ClickHouse: game_round_settled (高可用与多币种调优版)
-- 适用引擎: ClickHouse 22+
-- =============================================================================

CREATE DATABASE IF NOT EXISTS rgs_analytics;

DROP TABLE IF EXISTS rgs_analytics.game_round_settled;
CREATE TABLE rgs_analytics.game_round_settled (
    -- 1. 全局唯一业务凭据与主体索引
                                                  round_id String COMMENT '单局业务全局唯一号 (雪花ID / UUID)',
                                                  merchant_code LowCardinality(String) COMMENT '商户代号 (低基数字符串字典优化)',
                                                  game_code LowCardinality(String) COMMENT '游戏代号 (如: fishing_deep_sea, gold_miner)',
                                                  game_category LowCardinality(String) COMMENT '品类: ARCADE, SLOT, MINES, CASCADE',
                                                  user_id String COMMENT '玩家在下游系统的外部唯一ID',
                                                  currency LowCardinality(String) COMMENT '实际交易结算币种 (如: USD, BRL, PHP)',
                                                  is_demo UInt8 COMMENT '模式标识: 0-真金局, 1-虚拟试玩局',
                                                  is_promo UInt8 COMMENT '是否促销局: 0-正常自费局, 1-免费局/活动券抵扣',

    -- 2. 资金账变核心度量 (实际结算币种，精确到: 分)
                                                  bet_amount Int64 COMMENT '原币种总投注扣款金额 (分)',
                                                  win_amount Int64 COMMENT '原币种总派彩返奖金额 (分，0表示未中奖)',
                                                  net_profit Int64 COMMENT '商户端原币种毛利收入 (分): bet_amount - win_amount',
                                                  payout_multiplier Float32 COMMENT '实际产生倍率 (win_amount / bet_amount)',

    -- 3. 跨国基准多币种统一聚合 (以 USD 分为全平台锚定)
                                                  exchange_rate_usd Float32 COMMENT '结算发生时刻该币种对 USD 的锚定汇率 (1 Local = ? USD)',
                                                  bet_amount_usd Int64 COMMENT '折算为 USD 后的投注金额 (USD 分，用于全平台跨币种毫秒级聚合)',
                                                  win_amount_usd Int64 COMMENT '折算为 USD 后的派彩金额 (USD 分)',
                                                  net_profit_usd Int64 COMMENT '折算为 USD 后的平台毛利 (USD 分)',

    -- 4. 数学模型版本控制与审计 (高可用关键)
                                                  math_version LowCardinality(String) COMMENT '关联 game_math_models.math_version，锁定该局计算采用的静态 PAR 表版本',
                                                  rtp_tier_applied Float32 COMMENT '本次计算采用的标称理论 RTP 挡位 (如: 96.00)',

    -- 5. 确定性与可验证算法证据链 (Provably Fair 证据链)
                                                  server_seed String COMMENT '服务端随机数种子明文 (供事后验证)',
                                                  server_seed_hash String COMMENT '下注前已出示给前端的种子 SHA256 哈希',
                                                  client_seed String COMMENT '客户端种子/随机因子',
                                                  nonce UInt64 COMMENT '该种子对下的递增序列号 (Nonce)',

    -- 6. 业务特定视觉演播回放包 (全机台兼容容器)
                                                  presentation_payload String COMMENT '供前端完全还原出鱼/滚轮/连击/开箱的完整 JSON 序列化数据包',

    -- 7. 性能与链路审计
                                                  execution_time_us UInt32 COMMENT 'Go 核心数学推演计算耗时 (微秒)',
                                                  wallet_latency_ms UInt16 COMMENT '下游无缝钱包 Bet+Win 双向网络往返耗时 (毫秒)',
                                                  client_ip String COMMENT '发起抛竿的玩家终端真实 IP',
                                                  user_agent String COMMENT '玩家终端浏览器/设备 UA',
                                                  created_at DateTime DEFAULT now() COMMENT '注单最终落库时间 (UTC)'
)
    ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(created_at)
PRIMARY KEY (merchant_code, game_code, created_at)
ORDER BY (merchant_code, game_code, created_at, round_id)
SETTINGS index_granularity = 8192;

-- 针对 presentation_payload (大体积回放JSON) 实施 90 天过期自动抹除
-- 基础财务账变与倍数永久保留，兼顾审计合规与磁盘存储成本
ALTER TABLE rgs_analytics.game_round_settled
    MODIFY COLUMN presentation_payload String
    TTL created_at + INTERVAL 90 DAY;