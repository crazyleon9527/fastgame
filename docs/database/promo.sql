-- [玩家完成一次抛竿/下注]
-- │
--              ▼
-- [Go-Zero 核心管道完成计算]:
--   - 检查当前是否有生效中的 TOURNAMENT 活动
--   - 提取本局倍数 (如爆出了 120x)
-- │
--              ▼ (纯内存原子更新排名，耗时 < 0.2ms)
-- [Redis ZSet 执行原子排名更新]:
--   Key: `tourney:rank:{campaign_id}`
--   `ZADD tourney:rank:101 GT 120.00 "user_8831"`  (仅当大于历史最高时更新)
-- │
--     ┌────────┴────────────────────────────────────────┐
--     ▼ (异步推流)                                      ▼ (异步批量持久化)
-- [WebSocket / 长轮询网关]                     [Kafka 异步通道]
--   - 监测到前 10 名名次发生交替变更              - 批量刷盘同步到 `promo_user_progress`
--   - 向同房间 Cocos 客户端推送 Top 10 跑马灯      - 作为活动到期结算终榜的法律依据
--   - 激起其他大户玩家的追赶好胜心

-- =============================================================================
-- 模块五：营销促活与活动引擎中心 (Promotions & Retention Domain)
-- 适用引擎: MySQL 8.0+
-- 字符集: utf8mb4 / 排序规则: utf8mb4_unicode_ci
-- =============================================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------------------------
-- 1. 全局营销活动主档案表 (promo_campaigns)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `promo_campaigns`;
CREATE TABLE `promo_campaigns` (
                                   `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '活动自增主键ID',
                                   `campaign_code` VARCHAR(64) NOT NULL COMMENT '活动唯一业务代号 (如: TOURN_DEEPSEA_2026_Q3)',
                                   `campaign_type` VARCHAR(32) NOT NULL COMMENT '活动类型: TOURNAMENT (锦标赛), MISSION (局数挑战), MYSTERY_DROP (随机空投), FREE_ROUND (免费转/局)',
                                   `title` JSON NOT NULL COMMENT '多语言活动主标题: {"en": "Deep Sea Hunter Tourney", "zh": "深海猎手锦标赛"}',
                                   `description` JSON NOT NULL COMMENT '多语言活动规则与副标题富文本说明',
                                   `theme_skin_id` VARCHAR(32) NOT NULL DEFAULT 'DEFAULT' COMMENT 'Cocos 客户端挂载的活动主题/换肤包ID',

    -- 适用范围控制 (多租户隔离)
                                   `scope_type` VARCHAR(16) NOT NULL DEFAULT 'ALL' COMMENT '适用商户范围: ALL (全平台商户通用), INCLUDED (指定白名单商户), EXCLUDED (排除特定商户)',
                                   `merchant_scope_ids` JSON NULL COMMENT '商户ID数组白名单/黑名单 (如: [1, 5, 12])',
                                   `game_scope_codes` JSON NOT NULL COMMENT '参与该活动的游戏代号数组 (如: ["fishing_deep_sea", "gold_miner"])',

    -- 时间与时区 (底层硬性统一使用绝对 UTC 时间)
                                   `start_time` DATETIME NOT NULL COMMENT '活动开始时间戳 (UTC)',
                                   `end_time` DATETIME NOT NULL COMMENT '活动结束时间戳 (UTC)',
                                   `display_timezone` VARCHAR(32) NOT NULL DEFAULT 'UTC' COMMENT '前端面板展示默认转换的参考时区 (如: America/Sao_Paulo)',

    -- 预算池与资金熔断
                                   `budget_cap` BIGINT NOT NULL DEFAULT 0 COMMENT '活动总资金预算硬顶上限 (分，0表示不设限)',
                                   `total_spent` BIGINT NOT NULL DEFAULT 0 COMMENT '当前系统已实际派发的奖励总成本 (分)',
                                   `reward_currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '奖励计价基准货币',

                                   `status` VARCHAR(16) NOT NULL DEFAULT 'DRAFT' COMMENT '活动状态: DRAFT (草稿), SCHEDULED (已排期), RUNNING (进行中), PAUSED (已暂停), FINISHED (已结算完毕), CANCELLED (已废弃)',
                                   `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                   `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                   PRIMARY KEY (`id`),
                                   UNIQUE KEY `uk_campaign_code` (`campaign_code`),
                                   KEY `idx_status_timeline` (`status`, `start_time`, `end_time`),
                                   KEY `idx_type` (`campaign_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='全局营销活动主档案与多时区生命周期管理表';

-- -----------------------------------------------------------------------------
-- 2. 锦标赛/排行榜规则配置表 (promo_tournament_rules)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `promo_tournament_rules`;
CREATE TABLE `promo_tournament_rules` (
                                          `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                          `campaign_id` BIGINT UNSIGNED NOT NULL COMMENT '关联活动ID (promo_campaigns.id)',
                                          `score_calculation_type` VARCHAR(32) NOT NULL DEFAULT 'HIGHEST_MULTIPLIER' COMMENT '积分计分模型: HIGHEST_MULTIPLIER (单局最高倍数), TURNOVER_ACCUMULATED (累计投注总额), WIN_STREAK (连续命中次数), TOTAL_NET_WIN (总净赢额)',
                                          `min_qualifying_bet` BIGINT NOT NULL DEFAULT 50 COMMENT '参与该锦标赛的单注最低门槛 (分，如 50 = $0.50，低于此额不计入积分)',
                                          `leaderboard_size` INT UNSIGNED NOT NULL DEFAULT 100 COMMENT '前台展示的上榜总人数 (如 Top 100)',
                                          `prize_pool_type` VARCHAR(16) NOT NULL DEFAULT 'FIXED' COMMENT '奖池类型: FIXED (固定保底奖金), PROGRESSIVE (流水动态抽水注水奖池)',
                                          `total_prize_pool` BIGINT NOT NULL DEFAULT 0 COMMENT '总奖池固定金额 (分)',
                                          `prize_distribution` JSON NOT NULL COMMENT '排名奖励分配阶梯 JSON: [{"rank_from": 1, "rank_to": 1, "reward_type": "CASH", "amount": 100000}, {"rank_from": 2, "rank_to": 5, "reward_type": "CASH", "amount": 20000}]',
                                          `is_settled` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '活动到期是否已完成奖金自动计算派发: 0-未结算, 1-已完成派发',
                                          `settled_at` DATETIME NULL COMMENT '终榜实际结算时间戳',
                                          `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                          `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                          PRIMARY KEY (`id`),
                                          UNIQUE KEY `uk_campaign_id` (`campaign_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='排行榜锦标赛规则、计分模型与奖池分配配置表';

-- -----------------------------------------------------------------------------
-- 3. 任务与打卡体系定义表 (promo_mission_definitions)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `promo_mission_definitions`;
CREATE TABLE `promo_mission_definitions` (
                                             `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '任务主键ID',
                                             `campaign_id` BIGINT UNSIGNED NOT NULL COMMENT '关联主活动ID (promo_campaigns.id)',
                                             `mission_code` VARCHAR(64) NOT NULL COMMENT '任务唯一代号 (如: MISSION_SPIN_50_ROUNDS)',
                                             `mission_type` VARCHAR(32) NOT NULL COMMENT '任务类型: ROUND_COUNT (累计局数), TOTAL_BET (累计投注), HIT_SPECIFIC_FISH (钓起指定特定鱼种)',
                                             `target_game_code` VARCHAR(32) NOT NULL DEFAULT 'ALL' COMMENT '目标机台代号 (ALL为全游戏通用)',
                                             `threshold_value` BIGINT NOT NULL COMMENT '达成任务所需的目标数值 (如局数需达到 50，或投注额需达到 $100)',
                                             `min_single_bet` BIGINT NOT NULL DEFAULT 10 COMMENT '单次下注有效计数的最低金额限制 (分)',
                                             `reward_type` VARCHAR(16) NOT NULL DEFAULT 'BONUS_MONEY' COMMENT '达成后发放奖励类型: BONUS_MONEY (红利金), FREE_SPINS (免费抛竿券), TICKET (锦标赛门票)',
                                             `reward_amount` BIGINT NOT NULL COMMENT '奖励金额 (分) 或 赠送免费局数量',
                                             `reward_meta` JSON NULL COMMENT '奖励附加元数据 (如免费局对应的固定单注面额: {"free_bet_value": 20})',
                                             `sort_order` INT NOT NULL DEFAULT 0 COMMENT '任务展示排序',
                                             `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                             `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                             PRIMARY KEY (`id`),
                                             UNIQUE KEY `uk_mission_code` (`mission_code`),
                                             KEY `idx_campaign_sort` (`campaign_id`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='促活任务条件、达成门槛与激励定义表';

-- -----------------------------------------------------------------------------
-- 4. 玩家活动完成进度状态表 (promo_user_progress)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `promo_user_progress`;
CREATE TABLE `promo_user_progress` (
                                       `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
                                       `campaign_id` BIGINT UNSIGNED NOT NULL COMMENT '关联活动ID',
                                       `mission_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关联具体任务ID (0 表示锦标赛等无具体子任务)',
                                       `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '所属商户ID',
                                       `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户代号',
                                       `user_id` VARCHAR(64) NOT NULL COMMENT '下游玩家唯一代号',
                                       `current_score` DECIMAL(14,4) NOT NULL DEFAULT 0.0000 COMMENT '当前锦标赛积分 / 最高倍率快照',
                                       `accumulated_value` BIGINT NOT NULL DEFAULT 0 COMMENT '当前任务已累加进度数值 (如已完成 32 局)',
                                       `target_value` BIGINT NOT NULL DEFAULT 0 COMMENT '目标数值快照',
                                       `is_completed` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否已达成目标: 0-未完成, 1-已达成',
                                       `is_reward_claimed` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '奖励是否已被领取核销: 0-未领, 1-已领取',
                                       `last_action_time` DATETIME NOT NULL COMMENT '最后一次进度推进时间',
                                       `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次参与记录时间',
                                       `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                       PRIMARY KEY (`id`),
                                       UNIQUE KEY `uk_user_mission` (`campaign_id`, `mission_id`, `merchant_id`, `user_id`),
                                       KEY `idx_user_progress` (`merchant_id`, `user_id`, `is_completed`),
                                       KEY `idx_campaign_score` (`campaign_id`, `current_score` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='玩家营销活动实时进度与达成状态持久化表';

-- -----------------------------------------------------------------------------
-- 5. 活动奖励派发与核销流水表 (promo_reward_grants)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `promo_reward_grants`;
CREATE TABLE `promo_reward_grants` (
                                       `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '派发流水主键ID',
                                       `grant_id` VARCHAR(64) NOT NULL COMMENT '奖励发放全局唯一业务号 (UUID / 雪花ID)',
                                       `campaign_id` BIGINT UNSIGNED NOT NULL COMMENT '来源活动ID',
                                       `campaign_code` VARCHAR(64) NOT NULL COMMENT '来源活动代号',
                                       `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '商户ID',
                                       `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户代号',
                                       `user_id` VARCHAR(64) NOT NULL COMMENT '获赠玩家ID',
                                       `currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '奖励币种',
                                       `reward_type` VARCHAR(16) NOT NULL COMMENT '奖励类型: BONUS_MONEY (红利金), REAL_CASH (真金可提现), FREE_ROUNDS (免费游戏局)',
                                       `amount` BIGINT NOT NULL COMMENT '派发金额 (分) 或 免费局数',
                                       `wagering_multiplier` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '流水打码量要求倍数 (如 10.00 代表需达到 10 倍下注方可转为真金提现)',
                                       `wagering_required_amount` BIGINT NOT NULL DEFAULT 0 COMMENT '实际需要达到的绝对下注打码金额 (分): amount * wagering_multiplier',
                                       `wagering_current_amount` BIGINT NOT NULL DEFAULT 0 COMMENT '玩家当前已完成的累计打码金额 (分)',
                                       `target_wallet` VARCHAR(16) NOT NULL DEFAULT 'BONUS' COMMENT '打入目标账户: BONUS (商户红利钱包), REAL (主资金钱包), TECHNICAL_MOCK (仅本地虚拟金)',
                                       `status` VARCHAR(16) NOT NULL DEFAULT 'ISSUED' COMMENT '奖励状态: ISSUED (已发放到账), IN_WAGERING (打码中), COMPLETED (流水达标结清), EXPIRED (超时未完成没收)',
                                       `valid_until` DATETIME NOT NULL COMMENT '奖励有效期截止时间 (超时自动销毁)',
                                       `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '派发时间',
                                       `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                       PRIMARY KEY (`id`),
                                       UNIQUE KEY `uk_grant_id` (`grant_id`),
                                       KEY `idx_user_status` (`merchant_id`, `user_id`, `status`),
                                       KEY `idx_campaign_grant` (`campaign_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='营销奖励派发、打码量(Wagering)考核与核销总账表';

SET FOREIGN_KEY_CHECKS = 1;