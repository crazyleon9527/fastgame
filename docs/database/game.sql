-- =============================================================================
-- 模块二：游戏定义、单局状态机与核心账变流水 (Game & Transaction Domain)
-- 适用引擎: MySQL 8.0+
-- 字符集: utf8mb4 / 排序规则: utf8mb4_unicode_ci
-- =============================================================================

SET NAMES utf8mb4;
SET
FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------------------------
-- 1. 全局游戏字典注册表 (games)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `games`;
CREATE TABLE `games`
(
    `id`                  INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '游戏自增ID',
    `game_code`           VARCHAR(32)   NOT NULL COMMENT '游戏全局唯一代号 (如: fishing_deep_sea, gold_miner, slot_zodiac)',
    `category`            VARCHAR(16)   NOT NULL COMMENT '玩法品类: ARCADE (钓鱼/矿工), SLOT (多轴轮盘), MINES (步进扫雷), CASCADE (消除下落)',
    `title`               JSON          NOT NULL COMMENT '多语言游戏显示名称: {"en": "Fishing Tycoon", "zh": "钓鱼大亨"}',
    `icon_url`            VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '游戏主视觉 Icon 资源路径',
    `banner_url`          VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '游戏大厅横幅/海报图路径',
    `bundle_path`         VARCHAR(255)  NOT NULL DEFAULT '' COMMENT 'Cocos 资源分包加载相对路径 (Subpackage URL)',
    `engine_plugin_name`  VARCHAR(32)   NOT NULL COMMENT 'Go 核心数学插件注册名 (与代码中的 GamePlugin 映射)',
    `supported_rtp_tiers` JSON          NOT NULL COMMENT '该机台支持的预设 RTP 挡位数组: [94.00, 96.00, 98.00]',
    `default_rtp`         DECIMAL(5, 2) NOT NULL DEFAULT 96.00 COMMENT '系统默认激活的基准 RTP 挡位',
    `max_multiplier`      INT UNSIGNED NOT NULL DEFAULT 1000 COMMENT '系统设定的单局硬顶最大倍数 (风控安全防线)',
    `has_multi_step`      TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否为多阶段步进状态机游戏 (如扫雷): 0-否 (瞬时结算), 1-是 (需要会话暂存)',
    `display_order`       INT           NOT NULL DEFAULT 0 COMMENT '在大厅中的推荐排序权重 (越大越靠前)',
    `status`              VARCHAR(16)   NOT NULL DEFAULT 'ONLINE' COMMENT '状态: ONLINE (正常上线), COMING_SOON (敬请期待占位), MAINTENANCE (单游戏维护)',
    `created_at`          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_game_code` (`game_code`),
    KEY                   `idx_status_category` (`status`, `category`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='全局游戏机台字典注册与基础数学属性表';

-- -----------------------------------------------------------------------------
-- 2. 商户游戏授权与点控配置表 (merchant_games)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_games`;
CREATE TABLE `merchant_games`
(
    `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `merchant_id`        BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
    `merchant_code`      VARCHAR(32)   NOT NULL COMMENT '商户代号冗余',
    `game_id`            INT UNSIGNED NOT NULL COMMENT '关联游戏ID (games.id)',
    `game_code`          VARCHAR(32)   NOT NULL COMMENT '游戏代号冗余',
    `active_rtp`         DECIMAL(5, 2) NOT NULL DEFAULT 96.00 COMMENT '为该商户启用的当前实际 RTP 挡位',
    `min_bet`            BIGINT        NOT NULL DEFAULT 10 COMMENT '该商户下此游戏的单注底注 (分，10 = $0.10)',
    `max_bet`            BIGINT        NOT NULL DEFAULT 50000 COMMENT '该商户下此游戏的单注顶注 (分，50000 = $500.00)',
    `max_multiplier_cap` INT UNSIGNED NOT NULL DEFAULT 1000 COMMENT '商户定制的单局最大倍数硬顶上限',
    `status`             TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '授权状态: 0-停用下架, 1-正常开放, 2-商户专属维护',
    `created_at`         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_merchant_game` (`merchant_id`, `game_id`),
    KEY                  `idx_lookup` (`merchant_code`, `game_code`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户与游戏授权矩阵及点控配置表';

-- -----------------------------------------------------------------------------
-- 3. 多阶段游戏活动会话表 (game_sessions)
-- 适用场景: 《盗墓迷城》扫雷步进等需中途 Cashout 的玩法
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `game_sessions`;
CREATE TABLE `game_sessions`
(
    `id`                     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '会话主键ID',
    `session_id`             VARCHAR(64)    NOT NULL COMMENT '会话全局唯一ID (对应前台一次长局)',
    `round_id`               VARCHAR(64)    NOT NULL COMMENT '绑定的全局注单唯一号',
    `merchant_id`            BIGINT UNSIGNED NOT NULL COMMENT '商户ID',
    `merchant_code`          VARCHAR(32)    NOT NULL COMMENT '商户代号',
    `game_id`                INT UNSIGNED NOT NULL COMMENT '游戏ID',
    `game_code`              VARCHAR(32)    NOT NULL COMMENT '游戏代号',
    `user_id`                VARCHAR(64)    NOT NULL COMMENT '下游系统玩家唯一代号',
    `currency`               VARCHAR(8)     NOT NULL DEFAULT 'USD' COMMENT '货币代号',
    `bet_amount`             BIGINT         NOT NULL COMMENT '开局下注扣款本金 (分)',
    `current_step`           INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前已成功完成的步数 (步进深度)',
    `current_multiplier`     DECIMAL(10, 4) NOT NULL DEFAULT 1.0000 COMMENT '当前累积达到的倍率',
    `accumulated_payout`     BIGINT         NOT NULL DEFAULT 0 COMMENT '当前若选择结算可拿走的派彩金额 (分)',
    `step_history`           JSON NULL COMMENT '步进历史与状态快照: [{"step":1,"action":"tile_3","outcome":"safe","mult":1.2}]',
    `encrypted_secret_state` TEXT           NOT NULL COMMENT '加密存储的底牌/炸弹位置 (防内存透视或提前解密)',
    `step_version` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '步进操作版本号 (乐观锁)',
    `session_status`         VARCHAR(16)    NOT NULL DEFAULT 'IN_PROGRESS' COMMENT '会话状态: IN_PROGRESS (进行中), CASHED_OUT (主动提现), CRASHED (触雷失败), EXPIRED_AUTO_SETTLE (超时系统自动平账)',
    `expire_at`              DATETIME       NOT NULL COMMENT '会话最迟过期时间 (到期由 Cron 自动执行保底结算并销毁)',
    `created_at`             DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开局时间',
    `updated_at`             DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后操作更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_session_id` (`session_id`),
    UNIQUE KEY `uk_round_id` (`round_id`),
    KEY                      `idx_active_user` (`merchant_id`, `user_id`, `session_status`),
    KEY                      `idx_expire_sweep` (`session_status`, `expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='多阶段步进类游戏未完局状态机与断线恢复会话表';

-- -----------------------------------------------------------------------------
-- 4. 游戏核心资金账变流水表 (game_transactions) - 强一致性资金账本
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `game_transactions`;
CREATE TABLE `game_transactions`
(
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '账变自增主键ID',
    `transaction_id` VARCHAR(64)  NOT NULL COMMENT '我方系统全局唯一账变流水号 (雪花算法生成，TxID)',
    `external_tx_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '下游无缝钱包返回的三方对账凭据号 (External TxID)',
    `round_id`       VARCHAR(64)  NOT NULL COMMENT '归属单局业务唯一流水号 (Round ID)',
    `merchant_id`    BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID',
    `merchant_code`  VARCHAR(32)  NOT NULL COMMENT '商户代号冗余',
    `game_id`        INT UNSIGNED NOT NULL COMMENT '关联游戏ID',
    `game_code`      VARCHAR(32)  NOT NULL COMMENT '游戏代号冗余',
    `user_id`        VARCHAR(64)  NOT NULL COMMENT '玩家在下游系统的唯一ID',
    `currency`       VARCHAR(8)   NOT NULL DEFAULT 'USD' COMMENT '交易结算币种',
    `direction`      VARCHAR(8)   NOT NULL COMMENT '资金流向: IN (玩家进账/派彩), OUT (玩家出资/扣款)',
    `tx_type`        VARCHAR(16)  NOT NULL COMMENT '账变类型: BET (有效下注扣款), WIN (派彩中奖), REFUND (异常退款), ROLLBACK (撤单回滚), PROMO_CREDIT (活动空投)',
    `amount`         BIGINT       NOT NULL COMMENT '发生变动的绝对金额 (分，必须 > 0)',
    `balance_before` BIGINT       NOT NULL DEFAULT 0 COMMENT '账变前玩家余额快照 (若下游钱包返回则记录，分)',
    `balance_after`  BIGINT       NOT NULL DEFAULT 0 COMMENT '账变后玩家余额快照 (若下游钱包返回则记录，分)',
    `is_demo`        TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '模式标识: 0-真实货币账变, 1-虚拟试玩账变',
    `status`         VARCHAR(16)  NOT NULL DEFAULT 'SUCCESS' COMMENT '账变终态: SUCCESS (成功落账), FAILED (下游扣款失败), PENDING_RETRY (挂起进死信补偿)',
    `remark`         VARCHAR(255) NOT NULL DEFAULT '' COMMENT '账变原因/上下文备注',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '账变产生时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_tx_id` (`transaction_id`),
    KEY              `idx_round_id` (`round_id`),
    KEY              `idx_merchant_user` (`merchant_id`, `user_id`, `created_at`),
    KEY              `idx_external_tx` (`merchant_id`, `external_tx_id`),
    KEY              `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='游戏核心资金账变流水明细总账表 (Financial Transaction Ledger)';

-- -----------------------------------------------------------------------------
-- 5. 单局业务汇总与对账镜像表 (game_rounds) - 强事务对账镜像
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `game_rounds`;
CREATE TABLE `game_rounds` (
                               `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
                               `round_id` VARCHAR(64) NOT NULL COMMENT '单局业务全局唯一号 (全链路业务主键)',
                               `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID',
                               `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户唯一代号',
                               `game_id` INT UNSIGNED NOT NULL COMMENT '关联游戏ID',
                               `game_code` VARCHAR(32) NOT NULL COMMENT '游戏全局唯一代号',
                               `math_version` VARCHAR(32) NOT NULL DEFAULT 'v1.0.0' COMMENT '运算使用的数学模型版本号 (对齐 game_math_models)',
                               `user_id` VARCHAR(64) NOT NULL COMMENT '玩家在下游系统的唯一ID',
                               `currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '原始结算货币代号',

    -- 原币种资金度量 (分)
                               `bet_amount` BIGINT NOT NULL DEFAULT 0 COMMENT '原币种总下注金额 (分)',
                               `win_amount` BIGINT NOT NULL DEFAULT 0 COMMENT '原币种总派彩金额 (分，0为未中奖)',
                               `net_amount` BIGINT NOT NULL DEFAULT 0 COMMENT '平台单局净盈亏 (分): bet_amount - win_amount',
                               `payout_multiplier` DECIMAL(10,4) NOT NULL DEFAULT 0.0000 COMMENT '计算出的实际返还倍率 (win_amount / bet_amount)',

    -- 美元基准折算 (用于跨币种对账与财务月结)
                               `exchange_rate_usd` DECIMAL(14,6) NOT NULL DEFAULT 1.000000 COMMENT '对账发生时相对 USD 的快照汇率',
                               `bet_amount_usd` BIGINT NOT NULL DEFAULT 0 COMMENT '折算为 USD 的投注额 (USD 分)',
                               `win_amount_usd` BIGINT NOT NULL DEFAULT 0 COMMENT '折算为 USD 的派彩额 (USD 分)',
                               `net_amount_usd` BIGINT NOT NULL DEFAULT 0 COMMENT '折算为 USD 的净收益 (USD 分)',

                               `is_demo` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0: 真金局, 1: 试玩局',
                               `is_promo` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否为营销局 (如免费转/活动送局): 0-否, 1-是',
                               `settle_status` VARCHAR(16) NOT NULL DEFAULT 'SETTLED' COMMENT '单局生命周期终态: SETTLED (正常结清), CANCELLED (全额撤单退回), HELD_AUDIT (触发看门狗风控挂起)',
                               `started_at` DATETIME NOT NULL COMMENT '单局抛竿/起步时间',
                               `ended_at` DATETIME NOT NULL COMMENT '单局派彩结算完毕时间',
                               `duration_ms` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '从开始到结算内部处理总耗时 (毫秒)',
                               `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录入库时间',
                               PRIMARY KEY (`id`),
                               UNIQUE KEY `uk_round_id` (`round_id`),
                               KEY `idx_merchant_game_time` (`merchant_code`, `game_code`, `created_at`),
                               KEY `idx_user_rounds` (`merchant_id`, `user_id`, `created_at`),
                               KEY `idx_settle_status` (`settle_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='单局业务事实汇总与资金对账镜像表';
SET
FOREIGN_KEY_CHECKS = 1;