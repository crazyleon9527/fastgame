-- =============================================================================
-- 模块一：商户模块 (Merchant Domain) - 全套建表 DDL
-- 适用引擎: MySQL 8.0+
-- 字符集: utf8mb4 / 排序规则: utf8mb4_unicode_ci
-- =============================================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------------------------
-- 1. 商户主档案与租户主体表 (merchant_accounts)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_accounts`;
CREATE TABLE `merchant_accounts` (
                                     `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '商户主键ID',
                                     `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户唯一代号 (全局识别码，如: M_WINNER_01)',
                                     `name` VARCHAR(128) NOT NULL COMMENT '商户对外主体全称',
                                     `short_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '商户简称 (大厅显示或简短标识)',
                                     `company_reg_number` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '海外法人实体注册编号/商业执照号',
                                     `tier_level` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '商户层级: 1-直客, 2-主聚合商(Master Aggregator), 3-子分销渠道',
                                     `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '父级商户ID (针对分销与聚合层级，0为顶级主体)',
                                     `default_currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '法定基准对账币种 (ISO-4217 代码，如 USD, BRL, PHP)',
                                     `contact_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '主要商务/技术对接人姓名',
                                     `contact_email` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '核心对接邮箱',
                                     `contact_telegram` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Telegram 对接账号/群组',
                                     `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '商户主体状态: 0-待激活, 1-正常营业, 2-临时封禁/风控, 3-已注销',
                                     `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                     `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                     PRIMARY KEY (`id`),
                                     UNIQUE KEY `uk_merchant_code` (`merchant_code`),
                                     KEY `idx_status_tier` (`status`, `tier_level`),
                                     KEY `idx_parent_id` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='B2B商户档案与多租户主体表';

-- -----------------------------------------------------------------------------
-- 2. 商户接口凭证与加解密配置表 (merchant_credentials)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_credentials`;
CREATE TABLE `merchant_credentials` (
                                        `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                        `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                        `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户代号冗余',
                                        `environment` VARCHAR(16) NOT NULL DEFAULT 'PRODUCTION' COMMENT '环境分类: PRODUCTION (生产), STAGING (预发布), SANDBOX (沙盒测试)',
                                        `key_type` VARCHAR(16) NOT NULL DEFAULT 'HMAC_SHA256' COMMENT '鉴权算法: HMAC_SHA256, RSA2048, ED25519',
                                        `api_key` VARCHAR(64) NOT NULL COMMENT '对外公开的 Access Key ID / Client ID',
                                        `secret_key` VARCHAR(255) NOT NULL COMMENT '对称加密密钥 / 签名用 Secret Key',
                                        `public_key` TEXT NULL COMMENT '非对称加密之商户公钥 (RSA验签使用)',
                                        `private_key` TEXT NULL COMMENT '非对称加密之我方私钥 (密文存储或KMS引用)',
                                        `version` INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '密钥版本号 (用于不停机热轮换换密)',
                                        `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '凭证状态: 0-已失效, 1-生效中, 2-即将废除(过渡期双活)',
                                        `expired_at` DATETIME NULL COMMENT '凭证强制失效时间 (NULL为永久有效)',
                                        `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                        `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                        PRIMARY KEY (`id`),
                                        UNIQUE KEY `uk_api_key` (`api_key`),
                                        KEY `idx_merchant_env` (`merchant_id`, `environment`, `status`),
                                        KEY `idx_merchant_code` (`merchant_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户API接口安全凭证与密钥生命周期管理表';

-- -----------------------------------------------------------------------------
-- 3. 商户网络策略与环境拓扑表 (merchant_network_configs)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_network_configs`;
CREATE TABLE `merchant_network_configs` (
                                            `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                            `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                            `environment` VARCHAR(16) NOT NULL DEFAULT 'PRODUCTION' COMMENT '适用环境: PRODUCTION, STAGING, SANDBOX',
                                            `ip_whitelist` JSON NOT NULL COMMENT '允许调用我方网关的下游服务器 IPv4/IPv6 或 CIDR 数组',
                                            `egress_ips` JSON NULL COMMENT '我方出网回调下游时告知商户的固定出网 IP 列表',
                                            `geo_block_enabled` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否开启地理区域封锁: 0-关闭, 1-开启',
                                            `geo_blocked_countries` JSON NULL COMMENT '禁止访问该商户游戏的国家/地区二字码数组 (如: ["CN", "US", "SG"])',
                                            `rate_limit_rps` INT UNSIGNED NOT NULL DEFAULT 500 COMMENT '单商户在我方网关的并发频率上限 (Requests Per Second)',
                                            `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '策略生效状态: 0-停用, 1-启用',
                                            `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                            `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                            PRIMARY KEY (`id`),
                                            UNIQUE KEY `uk_merchant_env` (`merchant_id`, `environment`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户网络准入白名单与地域风控策略表';

-- -----------------------------------------------------------------------------
-- 4. 无缝钱包接口路由与健康度配置表 (merchant_wallet_endpoints)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_wallet_endpoints`;
CREATE TABLE `merchant_wallet_endpoints` (
                                             `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                             `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                             `environment` VARCHAR(16) NOT NULL DEFAULT 'PRODUCTION' COMMENT '适用环境: PRODUCTION, STAGING, SANDBOX',
                                             `base_url` VARCHAR(255) NOT NULL COMMENT '下游 Seamless 钱包基础通信 URL (如 https://api.wallet.com)',
                                             `path_get_balance` VARCHAR(128) NOT NULL DEFAULT '/api/wallet/balance' COMMENT '查询余额接口绝对/相对路径',
                                             `path_bet` VARCHAR(128) NOT NULL DEFAULT '/api/wallet/bet' COMMENT '原子扣款 (Bet) 接口路径',
                                             `path_win` VARCHAR(128) NOT NULL DEFAULT '/api/wallet/win' COMMENT '原子派彩 (Win) 接口路径',
                                             `path_rollback` VARCHAR(128) NOT NULL DEFAULT '/api/wallet/rollback' COMMENT '撤单回滚 (Rollback) 接口路径',
                                             `timeout_ms` INT UNSIGNED NOT NULL DEFAULT 1200 COMMENT 'HTTP 请求硬超时时间 (单位: 毫秒)',
                                             `retry_limit` TINYINT UNSIGNED NOT NULL DEFAULT 3 COMMENT '遇网络故障最大重试次数',
                                             `circuit_status` VARCHAR(16) NOT NULL DEFAULT 'HEALTHY' COMMENT '断路器当前健康度: HEALTHY (正常), DEGRADED (慢请求降级), TRIPPED (熔断阻断)',
                                             `fail_count_window` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前滑动窗口内累计失败通信次数',
                                             `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                             `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                             PRIMARY KEY (`id`),
                                             UNIQUE KEY `uk_merchant_env` (`merchant_id`, `environment`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户Seamless钱包API通信路由与熔断配置表';

-- -----------------------------------------------------------------------------
-- 5. 商户钱包通信死信与异步补偿流水表 (merchant_wallet_dlq)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_wallet_dlq`;
CREATE TABLE `merchant_wallet_dlq` (
                                       `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增流水主键ID',
                                       `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID',
                                       `round_id` VARCHAR(64) NOT NULL COMMENT '关联游戏全局唯一单局业务号 (业务幂等根基)',
                                       `action_type` VARCHAR(16) NOT NULL COMMENT '动作类型: BET (扣款), WIN (派彩), ROLLBACK (撤销)',
                                       `user_id` VARCHAR(64) NOT NULL COMMENT '玩家在下游系统的唯一代号',
                                       `amount` BIGINT NOT NULL COMMENT '涉及资金数额 (按货币最小单位: 分 存储)',
                                       `currency` VARCHAR(8) NOT NULL COMMENT '货币代号',
                                       `request_payload` JSON NOT NULL COMMENT '最后一次调用下游钱包时投递的完整 JSON 报文',
                                       `response_payload` TEXT NULL COMMENT '下游返回的错误报文、HTTP状态码或网络异常快照',
                                       `http_status` SMALLINT NOT NULL DEFAULT 0 COMMENT '最后一次通信 HTTP 状态码 (如: 504, 502, 0代表网络超时)',

    -- 重试生命周期与断路器控制 (高可用防雪崩)
                                       `retry_count` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行的异步重试次数',
                                       `max_retries` TINYINT UNSIGNED NOT NULL DEFAULT 10 COMMENT '允许的最大自动重试次数',
                                       `next_retry_time` DATETIME NOT NULL COMMENT '下次计划调度补偿的时间戳 (指数退避计算)',
                                       `status` VARCHAR(24) NOT NULL DEFAULT 'PENDING' COMMENT '补偿状态: PENDING (等待重试), PROCESSING (正在执行), SUCCESS (补偿成功), FAILED_FINAL (超限作废/需人工介入), MANUAL_FIXED (人工平账), SUSPENDED_BY_CIRCUIT (下游通道熔断暂停重试，待健康探测唤醒)',

                                       `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次入死信队列时间',
                                       `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后重试更新时间',
                                       PRIMARY KEY (`id`),
                                       UNIQUE KEY `uk_round_action` (`round_id`, `action_type`),
                                       KEY `idx_status_next_retry` (`status`, `next_retry_time`),
                                       KEY `idx_merchant_user` (`merchant_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='无缝钱包调用死信队列与断路器保护对账表';
-- -----------------------------------------------------------------------------
-- 6. 商户商务合同与分成阶梯表 (merchant_contracts)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_contracts`;
CREATE TABLE `merchant_contracts` (
                                      `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '合同主键ID',
                                      `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                      `contract_no` VARCHAR(64) NOT NULL COMMENT '商务纸质/电子合同唯一编号',
                                      `settlement_model` VARCHAR(16) NOT NULL DEFAULT 'GGR' COMMENT '商业结算模式: GGR (总博弈毛利抽成), NGR (扣减红利后抽成), FLAT_FEE (固定月保底)',
                                      `settlement_currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '合同法定义务结算货币',
                                      `base_rate` DECIMAL(5,4) NOT NULL DEFAULT 0.1200 COMMENT '基础技术服务分成比率 (如 0.1200 代表 12%)',
                                      `tiered_rates` JSON NULL COMMENT '阶梯费率配置 (如月流水超$100K降至10%: [{"min_ggr": 0, "max_ggr": 100000, "rate": 0.12}, {"min_ggr": 100001, "max_ggr": -1, "rate": 0.10}])',
                                      `monthly_min_guarantee` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '月度保底消费要求 (USD，若未达标按此补足)',
                                      `effective_start` DATETIME NOT NULL COMMENT '合同法律生效开始日期',
                                      `effective_end` DATETIME NOT NULL COMMENT '合同截止到期日期',
                                      `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '合同状态: 0-作废, 1-正常生效, 2-已过期',
                                      `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
                                      `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                      PRIMARY KEY (`id`),
                                      UNIQUE KEY `uk_contract_no` (`contract_no`),
                                      KEY `idx_merchant_effective` (`merchant_id`, `status`, `effective_start`, `effective_end`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户商务合同条款与收益分润规则阶梯表';

-- -----------------------------------------------------------------------------
-- 7. 商户日结与周期账单汇总表 (merchant_financial_periods)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_financial_periods`;
CREATE TABLE `merchant_financial_periods` (
                                              `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '账单主键ID',
                                              `period_code` VARCHAR(32) NOT NULL COMMENT '账期结算代号 (按日如: D_20260914_M01, 按月如: M_202609_M01)',
                                              `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID',
                                              `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户代号冗余',
                                              `period_type` VARCHAR(8) NOT NULL DEFAULT 'DAILY' COMMENT '账期类型: DAILY (日结), MONTHLY (月结账单)',
                                              `start_date` DATE NOT NULL COMMENT '统计区间开始日期 (UTC)',
                                              `end_date` DATE NOT NULL COMMENT '统计区间结束日期 (UTC)',
                                              `total_rounds` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '区间内总投注局数',
                                              `total_bet` BIGINT NOT NULL DEFAULT 0 COMMENT '总有效投注金额 (分)',
                                              `total_win` BIGINT NOT NULL DEFAULT 0 COMMENT '总有效派彩金额 (分)',
                                              `ggr` BIGINT NOT NULL DEFAULT 0 COMMENT '博弈毛利总额 (分): total_bet - total_win',
                                              `promo_cost` BIGINT NOT NULL DEFAULT 0 COMMENT '商户本期消耗的活动补贴成本 (分)',
                                              `share_rate_applied` DECIMAL(5,4) NOT NULL DEFAULT 0.0000 COMMENT '本账期最终核算的有效分成比率',
                                              `fee_receivable` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '我方技术服务费应收总额 (结算币种法币金额)',
                                              `settlement_currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '账单计价货币',
                                              `reconciliation_status` VARCHAR(16) NOT NULL DEFAULT 'UNCONFIRMED' COMMENT '对账审核状态: UNCONFIRMED (未确认), BALANCED (平账无误), DISCREPANCY (存在差异争议), SETTLED (已打款结清)',
                                              `confirmed_at` DATETIME NULL COMMENT '双方确认平账时间',
                                              `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '快照生成时间',
                                              `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                              PRIMARY KEY (`id`),
                                              UNIQUE KEY `uk_period_merchant` (`period_code`, `merchant_id`),
                                              KEY `idx_merchant_dates` (`merchant_id`, `start_date`, `end_date`),
                                              KEY `idx_recon_status` (`reconciliation_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户日终对账与财务分成周期账单表';

-- -----------------------------------------------------------------------------
-- 8. 商户对账差异明细表 (merchant_reconciliation_diffs)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_reconciliation_diffs`;
CREATE TABLE `merchant_reconciliation_diffs` (
                                                 `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '差异主键ID',
                                                 `period_id` BIGINT UNSIGNED NOT NULL COMMENT '关联周期账单ID (merchant_financial_periods.id)',
                                                 `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID',
                                                 `round_id` VARCHAR(64) NOT NULL COMMENT '引发争议的单局流水号',
                                                 `diff_type` VARCHAR(16) NOT NULL COMMENT '差异类型: MISSING_IN_UPSTREAM (下游有但我方无), MISSING_IN_DOWNSTREAM (我方有但下游无), AMOUNT_MISMATCH (双方金额不符)',
                                                 `our_bet` BIGINT NOT NULL DEFAULT 0 COMMENT '我方记录的投注额 (分)',
                                                 `our_win` BIGINT NOT NULL DEFAULT 0 COMMENT '我方记录的派彩额 (分)',
                                                 `their_bet` BIGINT NOT NULL DEFAULT 0 COMMENT '下游上报对账单中的投注额 (分)',
                                                 `their_win` BIGINT NOT NULL DEFAULT 0 COMMENT '下游上报对账单中的派彩额 (分)',
                                                 `diff_amount` BIGINT NOT NULL DEFAULT 0 COMMENT '绝对净差异金额 (分)',
                                                 `resolution_status` VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT '解决处理状态: PENDING (待核查), AUTO_RESOLVED (自动对账修补), MANUAL_ADJUSTED (人工确认调整), IGNORED (忽略小额尾差)',
                                                 `resolution_remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '财务/技术人员平账备注说明',
                                                 `resolved_at` DATETIME NULL COMMENT '平账了结时间戳',
                                                 `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发现差异并入库时间',
                                                 `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                                 PRIMARY KEY (`id`),
                                                 UNIQUE KEY `uk_period_round` (`period_id`, `round_id`),
                                                 KEY `idx_merchant_diff_status` (`merchant_id`, `resolution_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='自动化财务对账差异与单边账明细审计表';

-- -----------------------------------------------------------------------------
-- 9. 商户级独立风控策略表 (merchant_risk_policies)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_risk_policies`;
CREATE TABLE `merchant_risk_policies` (
                                          `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                          `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                          `max_single_bet` BIGINT NOT NULL DEFAULT 50000 COMMENT '单注投注硬上限 (分，如 50000 = $500.00)',
                                          `min_single_bet` BIGINT NOT NULL DEFAULT 10 COMMENT '单注投注硬下限 (分，如 10 = $0.10)',
                                          `max_single_win` BIGINT NOT NULL DEFAULT 25000000 COMMENT '单局派彩硬顶上限 (分，如 25000000 = $250,000.00，超额系统自动截断拦截)',
                                          `hourly_loss_limit` BIGINT NOT NULL DEFAULT 1000000 COMMENT '商户池1小时最大允许累计净亏损熔断额 (分，如 -$10,000 则触发商户级熔断)',
                                          `daily_loss_limit` BIGINT NOT NULL DEFAULT 5000000 COMMENT '商户池24小时最大允许累计净亏损熔断额 (分)',
                                          `max_user_concurrent_bets` TINYINT UNSIGNED NOT NULL DEFAULT 2 COMMENT '单玩家同时并发抛竿/下注最大允许线程数 (防并发刷单脚本)',
                                          `auto_suspend_on_leak` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '当算法检测到疑似数学穿池时是否自动挂起该商户: 0-仅告警, 1-毫秒级自动挂起',
                                          `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '策略状态: 0-停用, 1-启用',
                                          `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                          `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                          PRIMARY KEY (`id`),
                                          UNIQUE KEY `uk_merchant_id` (`merchant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户专属独立风控门槛与防击穿熔断策略表';

-- -----------------------------------------------------------------------------
-- 10. 商户活动营销授权与预算池配置表 (merchant_promo_configs)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_promo_configs`;
CREATE TABLE `merchant_promo_configs` (
                                          `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
                                          `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                          `promo_suite_enabled` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否启用技术方促活营销套件: 0-关闭, 1-开启',
                                          `allow_tournaments` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否允许参与锦标排行榜活动: 0-否, 1-是',
                                          `allow_missions` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否允许局数/打卡任务系统: 0-否, 1-是',
                                          `allow_drop_rewards` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否允许深海随机掉宝/彩蛋空投: 0-否, 1-是',
                                          `monthly_budget_cap` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '月度营销活动赞助/补贴预算上限 (USD，0表示不设限)',
                                          `current_month_spent` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '本月系统已实际派发的活动奖励累计总成本 (USD)',
                                          `reward_target_wallet` VARCHAR(16) NOT NULL DEFAULT 'BONUS' COMMENT '活动奖励派发落入的目标账户: BONUS (下游红利钱包), REAL (下游真实可提现主钱包)',
                                          `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '配置状态: 0-停用, 1-启用',
                                          `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                          `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                          PRIMARY KEY (`id`),
                                          UNIQUE KEY `uk_merchant_id` (`merchant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户营销套件特权授权与月度补贴预算池表';

-- -----------------------------------------------------------------------------
-- 11. 商户自服务后台管理员账号表 (merchant_users)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_users`;
CREATE TABLE `merchant_users` (
                                  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '后台用户ID',
                                  `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '归属商户ID (merchant_accounts.id)',
                                  `username` VARCHAR(64) NOT NULL COMMENT '登录账号用户名',
                                  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希值 (Argon2id 或 Bcrypt 加密)',
                                  `salt` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '密码混淆盐值',
                                  `real_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '使用者真实姓名/代号',
                                  `role` VARCHAR(32) NOT NULL DEFAULT 'VIEWER' COMMENT '权限角色: ADMIN (商户超管), DEVELOPER (技术对接), FINANCE (财务核账), VIEWER (只读观察员)',
                                  `totp_secret` VARCHAR(64) NULL COMMENT '谷歌双因子认证 (2FA / TOTP) 密钥密文',
                                  `is_2fa_enabled` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否强制开启 2FA: 0-未开启, 1-已开启',
                                  `last_login_ip` VARCHAR(45) NOT NULL DEFAULT '' COMMENT '最后一次成功登录的 IP 地址',
                                  `last_login_time` DATETIME NULL COMMENT '最后一次成功登录时间',
                                  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '用户状态: 0-冻结停用, 1-正常有效',
                                  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
                                  PRIMARY KEY (`id`),
                                  UNIQUE KEY `uk_merchant_username` (`merchant_id`, `username`),
                                  KEY `idx_merchant_role` (`merchant_id`, `role`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Sponge商户自服务后台管理员账号与RBAC权限表';

-- -----------------------------------------------------------------------------
-- 12. 商户敏感配置变更审计流水表 (merchant_audit_logs)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `merchant_audit_logs`;
CREATE TABLE `merchant_audit_logs` (
                                       `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '审计自增主键ID',
                                       `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '归属商户ID',
                                       `operator_type` VARCHAR(16) NOT NULL COMMENT '操作人身份类型: PLATFORM_ADMIN (我方平台运维), MERCHANT_USER (商户自主管理人员), SYSTEM_CRON (系统定时任务)',
                                       `operator_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '操作人员用户主键ID (0表示系统触发)',
                                       `operator_name` VARCHAR(64) NOT NULL DEFAULT 'SYSTEM' COMMENT '操作人账号快照',
                                       `action_module` VARCHAR(32) NOT NULL COMMENT '变更模块: CREDENTIALS (换密), NETWORK (白名单变更), RISK (风控修改), WALLET (路由调整)',
                                       `action_type` VARCHAR(32) NOT NULL COMMENT '操作动作: CREATE, UPDATE, DELETE, ROTATE_KEY, TOGGLE_STATUS',
                                       `target_table` VARCHAR(64) NOT NULL COMMENT '受影响的目标数据物理表名',
                                       `target_record_id` BIGINT UNSIGNED NOT NULL COMMENT '受影响的目标记录主键ID',
                                       `before_payload` JSON NULL COMMENT '修改前数据原始快照 (JSON 序列化存储)',
                                       `after_payload` JSON NULL COMMENT '修改后数据最新快照 (JSON 序列化存储)',
                                       `request_ip` VARCHAR(45) NOT NULL DEFAULT '' COMMENT '发起操作的客户端真实 IP',
                                       `user_agent` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '发起操作的客户端浏览器/终端 UA',
                                       `remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '修改原因或工单编号备注',
                                       `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '审计日志产生时间',
                                       PRIMARY KEY (`id`),
                                       KEY `idx_merchant_module` (`merchant_id`, `action_module`, `created_at`),
                                       KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户核心资产与敏感配置变更防篡改审计流水表';

SET FOREIGN_KEY_CHECKS = 1;