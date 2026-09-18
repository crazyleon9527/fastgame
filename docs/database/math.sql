DROP TABLE IF EXISTS `system_exchange_rates`;
CREATE TABLE `system_exchange_rates` (
                                         `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
                                         `base_currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '基准锚定货币 (通常为 USD)',
                                         `target_currency` VARCHAR(8) NOT NULL COMMENT '目标货币代码 (如: BRL, PHP, EUR, USDT)',
                                         `rate` DECIMAL(14,6) NOT NULL COMMENT '当前实时买入基准汇率 (1 USD = ? Target)',
                                         `inverse_rate` DECIMAL(14,6) NOT NULL COMMENT '倒数汇率 (1 Target = ? USD)',
                                         `source` VARCHAR(32) NOT NULL DEFAULT 'OPEN_EXCHANGE' COMMENT '汇率数据源',
                                         `effective_date` DATE NOT NULL COMMENT '生效基准日期 (UTC)',
                                         `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '同步入库时间',
                                         `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                         PRIMARY KEY (`id`),
                                         UNIQUE KEY `uk_currency_date` (`base_currency`, `target_currency`, `effective_date`),
                                         KEY `idx_lookup` (`target_currency`, `effective_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='全球多法币对基准USD实时与历史结算汇率表';

DROP TABLE IF EXISTS `player_game_sessions`;
CREATE TABLE `player_game_sessions` (
                                        `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
                                        `session_token` VARCHAR(64) NOT NULL COMMENT '前台游戏运行时握手 Token (UUID/安全随机串)',
                                        `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '所属商户ID',
                                        `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户代号',
                                        `user_id` VARCHAR(64) NOT NULL COMMENT '下游玩家唯一ID',
                                        `user_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '玩家显示昵称 (脱敏)',
                                        `game_id` INT UNSIGNED NOT NULL COMMENT '进入的目标游戏ID',
                                        `game_code` VARCHAR(32) NOT NULL COMMENT '目标游戏代号',
                                        `currency` VARCHAR(8) NOT NULL DEFAULT 'USD' COMMENT '玩家当前进入携带的货币代码',
                                        `is_demo` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否试玩会话: 0-真金, 1-试玩',
                                        `lang` VARCHAR(16) NOT NULL DEFAULT 'en-US' COMMENT '玩家指定的初始语种代码',
                                        `client_ip` VARCHAR(45) NOT NULL DEFAULT '' COMMENT '商户请求换取Token时的玩家IP',
                                        `device_type` VARCHAR(16) NOT NULL DEFAULT 'MOBILE' COMMENT '终端类型: MOBILE, DESKTOP, TABLET',
                                        `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '会话状态: 1-有效激活, 2-已被顶号踢出, 3-已自然过期',
                                        `expire_at` DATETIME NOT NULL COMMENT 'Token 硬性过期时间戳 (通常为生成后 24 小时)',
                                        `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '换取启动票据时间',
                                        `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                        PRIMARY KEY (`id`),
                                        UNIQUE KEY `uk_session_token` (`session_token`),
                                        KEY `idx_active_user` (`merchant_id`, `user_id`, `status`),
                                        KEY `idx_expire` (`expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='玩家进入机台运行态鉴权票据与顶号互踢会话表';

DROP TABLE IF EXISTS `game_math_models`;
CREATE TABLE `game_math_models` (
                                    `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
                                    `game_id` INT UNSIGNED NOT NULL COMMENT '关联游戏ID (games.id)',
                                    `game_code` VARCHAR(32) NOT NULL COMMENT '游戏代号',
                                    `math_version` VARCHAR(32) NOT NULL COMMENT '数学模型版本号 (如: v1.0.0_tier96)',
                                    `rtp_tier` DECIMAL(5,2) NOT NULL COMMENT '对应的标称理论 RTP (如 96.00)',
                                    `par_table_checksum` VARCHAR(64) NOT NULL COMMENT '权重表静态数组的 SHA256 指纹 (证明未被篡改)',
                                    `par_table_content` JSON NOT NULL COMMENT '完整的离散概率与出奖倍率映射数组配置',
                                    `simulation_spins` BIGINT UNSIGNED NOT NULL DEFAULT 100000000 COMMENT '上线前计算机模拟压测局数 (如1亿局)',
                                    `simulation_actual_rtp` DECIMAL(5,4) NOT NULL COMMENT '模拟测试实际跑出的收敛 RTP (如 0.9599)',
                                    `certified_by` VARCHAR(64) NOT NULL DEFAULT 'INTERNAL' COMMENT '算法认证机构或内部签名 (如: GLI, BMM, INTERNAL)',
                                    `is_active` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否可用: 0-停用, 1-现行生效',
                                    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                    PRIMARY KEY (`id`),
                                    UNIQUE KEY `uk_game_version` (`game_id`, `math_version`),
                                    KEY `idx_lookup` (`game_code`, `rtp_tier`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='游戏数学引擎PAR概率表版本化审计与认证档案表';