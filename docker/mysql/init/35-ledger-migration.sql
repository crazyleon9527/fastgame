-- 35-ledger-migration.sql
-- 账变体系落地：账变类型表 + 账变流水表 + 影子账户表
--
-- 【背景】
--   fastgame 之前没有任何账变流水：玩家余额在"商户钱包"（外部 HTTP 服务）侧，
--   本地只留下了失败补偿的痕迹（wallet_pending_ops / pending_transactions），
--   以及 ClickHouse 里的结算明细。于是"这笔钱为什么扣了""某天流水对不上"
--   只能三处对账拼，且没有任何一笔带前后余额快照。
--
--   goctl 生成的 internal/model/biz/gameTransactionsModel*.go 早就设计了
--   game_transactions（direction/tx_type/amount/balance_before/after/status），
--   但既没有建表 SQL 也没有任何代码引用——本迁移把它真正落地。
--
-- 【参考 platform-api 的设计，补上它缺的三件】
--   1) 账变类型表 transaction_types：把 tx_type/direction 从硬编码字符串
--      变成数据行。核心是「变动方向是数据，不是代码」——
--      调用方只传 type_code + 正数金额，由类型表决定加/减可用余额还是冻结余额。
--      这样调用点不可能写错符号，新增业务类型只要插一行数据。
--   2) 每笔流水都带前后余额快照（balance_before/after），
--      对账不必从头累加，任意时间点都能核对。
--   3) 唯一收口（见 pkg/ledger）：锁账户 → 算余额 → 写流水 → 更新快照。
--
-- 【为什么是"影子账"而不是本地权威账本】
--   玩家余额的权威在商户钱包侧，fastgame 不拥有它。因此 player_accounts
--   里的余额是**钱包返回值的镜像快照**，用于：
--     - 校验连续性：本笔 balance_before 必须等于上一笔该玩家的 balance_after；
--     - 发现不可解释差额：钱包返回的余额 != 本地按类型推算的余额时，
--       把差额记进 game_transactions.drift_minor（详见该列注释）。
--   钱的主链路一行不改，仍然以钱包为准。

USE fastgame;

SET NAMES utf8mb4;

-- ================================================================
-- 1. 账变类型表
-- ================================================================
CREATE TABLE IF NOT EXISTS `transaction_types` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `code`            VARCHAR(48)  NOT NULL COMMENT '程序代码中使用的唯一标识（如 BET/WIN/REFUND）',
  `scope`           VARCHAR(16)  NOT NULL DEFAULT 'player' COMMENT '账变主体：player=玩家 / merchant=商户',
  `io_type`         VARCHAR(8)   NOT NULL COMMENT '收支方向：IN=进账 / OUT=出账 / NONE=不计收支',
  `balance_change`  VARCHAR(16)  NOT NULL COMMENT '对可用余额的影响：INCREASE / DECREASE / NONE',
  `frozen_change`   VARCHAR(16)  NOT NULL DEFAULT 'NONE' COMMENT '对冻结余额的影响：INCREASE / DECREASE / NONE',
  `name`            VARCHAR(64)  NOT NULL COMMENT '中文名称（后台展示）',
  `name_i18n`       JSON         DEFAULT NULL COMMENT '多语言名称，形如 {"zh_CN":"下注","en":"Bet"}',
  `description`     VARCHAR(256) DEFAULT NULL COMMENT '说明',
  `status`          TINYINT      NOT NULL DEFAULT 1 COMMENT '0=停用 1=启用（停用后新账变会被熔断拒绝）',
  `created_at`      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_scope_status` (`scope`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账变类型定义表：变动方向由数据决定，调用方只传正数金额';

-- 种子类型：只放当前真正会写入的类型。
-- 商户维度（结算/佣金）等接入时再补，避免出现"有类型但没人写"的悬空配置。
INSERT INTO `transaction_types`
  (`code`, `scope`, `io_type`, `balance_change`, `frozen_change`, `name`, `name_i18n`, `description`)
VALUES
  ('BET',          'player', 'OUT',  'DECREASE', 'NONE', '下注扣款',   JSON_OBJECT('zh_CN','下注扣款','en','Bet debit'),        '有效下注，向钱包发起扣款并确认成功'),
  ('WIN',          'player', 'IN',   'INCREASE', 'NONE', '派彩中奖',   JSON_OBJECT('zh_CN','派彩中奖','en','Win payout'),        '结算派彩入账'),
  ('REFUND',       'player', 'IN',   'INCREASE', 'NONE', '异常退款',   JSON_OBJECT('zh_CN','异常退款','en','Refund'),            '补偿链路确认后的退款入账'),
  ('ROLLBACK',     'player', 'IN',   'INCREASE', 'NONE', '撤单回滚',   JSON_OBJECT('zh_CN','撤单回滚','en','Rollback'),          '孤儿注单撤单，把已扣的下注额退回玩家'),
  ('PROMO_CREDIT', 'player', 'IN',   'INCREASE', 'NONE', '活动赠送',   JSON_OBJECT('zh_CN','活动赠送','en','Promo credit'),      '活动空投/彩金入账（暂无业务写入，预置类型）'),
  ('ADJUST_ADD',   'player', 'IN',   'INCREASE', 'NONE', '人工加款',   JSON_OBJECT('zh_CN','人工加款','en','Manual credit'),     '后台人工加款，必须带操作人与原因'),
  ('ADJUST_SUB',   'player', 'OUT',  'DECREASE', 'NONE', '人工扣款',   JSON_OBJECT('zh_CN','人工扣款','en','Manual debit'),      '后台人工扣款，必须带操作人与原因')
ON DUPLICATE KEY UPDATE
  `io_type` = VALUES(`io_type`),
  `balance_change` = VALUES(`balance_change`),
  `frozen_change` = VALUES(`frozen_change`),
  `name` = VALUES(`name`),
  `name_i18n` = VALUES(`name_i18n`),
  `description` = VALUES(`description`);

-- ================================================================
-- 2. 账变流水表（落地 biz 模型里早已设计好的 game_transactions）
-- ================================================================
CREATE TABLE IF NOT EXISTS `game_transactions` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '账变自增主键 ID',
  `transaction_id`    CHAR(36)     NOT NULL COMMENT '我方全局唯一账变流水号（UUID v7）',
  `external_tx_id`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '下游钱包返回的三方对账凭据号（人工调账时填钱包侧凭证号）',
  `type_id`           BIGINT UNSIGNED NOT NULL COMMENT '账变类型 ID（transaction_types.id）',
  `tx_type`           VARCHAR(48)  NOT NULL COMMENT '账变类型编码冗余，便于按类型查询与出账',
  `direction`         VARCHAR(8)   NOT NULL COMMENT '资金流向冗余：IN=玩家进账 / OUT=玩家出账',
  `merchant_id`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '商户 ID（merchants.id），唯一键前导列',
  `merchant_code`     VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '商户编码冗余',
  `user_id`           VARCHAR(64)  NOT NULL COMMENT '玩家在下游系统的唯一 ID',
  `game_id`           BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '游戏 ID（games.id），非游戏账变为 0',
  `game_code`         VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '游戏编码冗余',
  `round_id`          VARCHAR(64)  DEFAULT NULL COMMENT '归属单局流水号；人工调账等无单局账变为 NULL',
  `currency`          VARCHAR(8)   NOT NULL DEFAULT 'USD' COMMENT '交易结算币种',
  `amount`            BIGINT       NOT NULL COMMENT '发生变动的绝对金额（minor units，scale=10000，必须 > 0）',
  `balance_before`    BIGINT       NOT NULL DEFAULT 0 COMMENT '账变前玩家余额快照（minor units）',
  `balance_after`     BIGINT       NOT NULL DEFAULT 0 COMMENT '账变后玩家余额快照（minor units，钱包返回则以钱包为准）',
  `frozen_before`     BIGINT       NOT NULL DEFAULT 0 COMMENT '账变前冻结余额快照（minor units，玩家维度一般为 0）',
  `frozen_after`      BIGINT       NOT NULL DEFAULT 0 COMMENT '账变后冻结余额快照（minor units）',
  `drift_minor`       BIGINT       NOT NULL DEFAULT 0 COMMENT '不可解释差额：钱包返回余额 - 本地按类型推算余额。非 0 说明本地流水与钱包对不上，需人工核查',
  `is_demo`           TINYINT      NOT NULL DEFAULT 0 COMMENT '0=真实货币 1=虚拟试玩',
  `status`            VARCHAR(16)  NOT NULL COMMENT '账变终态：SUCCESS=成功落账 / FAILED=钱包明确失败 / PENDING_RETRY=待补偿重试',
  `remark`            VARCHAR(255) NOT NULL DEFAULT '' COMMENT '账变原因/上下文备注',
  `extra_data`        JSON         DEFAULT NULL COMMENT '扩展快照（操作人、补偿来源、请求参数摘要等）',
  `created_at`        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '账变产生时间（UTC）',
  `updated_at`        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_transaction_id` (`transaction_id`),
  -- 幂等靠它：同一商户同一局同一类型只允许一条，重放不会重复扣钱。
  -- round_id 为 NULL（人工调账）时 MySQL 唯一索引视为互不相同，因此不受约束。
  UNIQUE KEY `uk_merchant_round_type` (`merchant_id`, `round_id`, `tx_type`),
  KEY `idx_user_created` (`merchant_id`, `user_id`, `id`),
  KEY `idx_type_created` (`type_id`, `id`),
  KEY `idx_round` (`round_id`),
  KEY `idx_external_tx` (`external_tx_id`),
  KEY `idx_drift` (`drift_minor`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账变流水表：每笔余额变动一条，带前后余额快照';

-- ================================================================
-- 3. 影子账户表（钱包余额的本地镜像，非权威）
-- ================================================================
CREATE TABLE IF NOT EXISTS `player_accounts` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
  `merchant_id`       BIGINT UNSIGNED NOT NULL COMMENT '商户 ID（merchants.id），唯一键前导列',
  `merchant_code`     VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '商户编码冗余',
  `user_id`           VARCHAR(64)  NOT NULL COMMENT '玩家 ID',
  `currency`          VARCHAR(8)   NOT NULL DEFAULT 'USD' COMMENT '币种',
  `balance_minor`     BIGINT       NOT NULL DEFAULT 0 COMMENT '可用余额镜像快照（minor units），以钱包返回值为准',
  `frozen_minor`      BIGINT       NOT NULL DEFAULT 0 COMMENT '冻结余额镜像快照（minor units），平台预留',
  `version`           BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '版本号，用于乐观锁二次兜底',
  `last_ledger_id`    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最后一条账变流水 ID，用于连续性校验',
  `drift_count`       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '累计发现不可解释差额的次数',
  `last_drift_minor`  BIGINT       NOT NULL DEFAULT 0 COMMENT '最近一次不可解释差额',
  `last_drift_at`     DATETIME(3)  DEFAULT NULL COMMENT '最近一次发现差额的时间',
  `created_at`        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_merchant_user` (`merchant_id`, `user_id`, `currency`),
  KEY `idx_drift_count` (`drift_count`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='玩家账户影子表：余额为钱包返回值的镜像快照，钱包仍是权威';

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('35-ledger', 'Land the ledger: transaction_types + game_transactions + player_accounts');

SELECT '35-ledger-migration applied' AS note;
