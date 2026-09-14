-- [运营/翻译在 Sponge 后台保存文案]
-- │
--                 ▼ (点击“发布语言包”)
-- [后台编译流水线 Compiler]:
--   1. 合并 i18n_messages + i18n_merchant_overrides
--   2. 生成紧凑型 JSON 文件并上传至 S3/Cloudflare CDN (如 /i18n/v1.2/fishing_en-US.json)
--   3. 将全量字典写入 Redis Hash 预热 (供 Go 后端毫秒级读取)
-- │
--     ┌───────────┴────────────────────────────────┐
--     ▼ (服务端接口报错与提示)                       ▼ (客户端画面文字展示)
-- [Go-Zero 核心网关 / 拦截器]                [Cocos 客户端启动 / 进入机台]
--   - 收到请求，解析 Header `lang=pt-BR`      - 检查本地 localStorage 缓存的 hash
--   - 业务抛出错误码: `ERR_BALANCE_LOW`       - 若服务端下发新版本，异步拉取 CDN 静态 JSON
--   - 直接从 Redis 纯内存 Hash 获取翻译:      - 内存挂载 i18n.t("btn_spin")，零延迟极速渲染
--     `HGET cache:i18n:pt-BR ERR_BALANCE_LOW`
--   - 动态替换参数后返回给用户 (耗时 < 0.1ms)


-- =============================================================================
-- 模块四：多语言与动态本地化中心 (I18n & Localization Domain)
-- 适用引擎: MySQL 8.0+
-- 字符集: utf8mb4 / 排序规则: utf8mb4_unicode_ci
-- =============================================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------------------------
-- 1. 系统受支持语种字典表 (i18n_languages)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `i18n_languages`;
CREATE TABLE `i18n_languages` (
                                  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
                                  `lang_code` VARCHAR(16) NOT NULL COMMENT '标准语言标签 (BCP 47 规范，如: en-US, zh-CN, pt-BR, es-419, vi-VN, tl-PH)',
                                  `name_local` VARCHAR(64) NOT NULL COMMENT '该语言的本族语名称 (如: English, 简体中文, Português, Español)',
                                  `name_en` VARCHAR(64) NOT NULL COMMENT '该语言的标准英文全称 (如: Brazilian Portuguese)',
                                  `text_direction` VARCHAR(4) NOT NULL DEFAULT 'LTR' COMMENT '文字书写排版方向: LTR (从左到右), RTL (从右到左，如阿语/希伯来语)',
                                  `fallback_lang` VARCHAR(16) NOT NULL DEFAULT 'en-US' COMMENT '当某个词条缺失翻译时的自动降级回退语种',
                                  `sort_order` INT NOT NULL DEFAULT 0 COMMENT '前端语言切换下拉菜单的展示排序 (越大越靠前)',
                                  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '启用状态: 0-禁用隐藏, 1-正式启用, 2-开发测试中',
                                  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                  PRIMARY KEY (`id`),
                                  UNIQUE KEY `uk_lang_code` (`lang_code`),
                                  KEY `idx_status_sort` (`status`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='受支持国际化语种定义与文本排版属性表';

-- -----------------------------------------------------------------------------
-- 2. 多语言模块命名空间表 (i18n_modules)
-- 作用: 避免前端一次性拉取数十万行全部翻译，按业务按模块按需下发
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `i18n_modules`;
CREATE TABLE `i18n_modules` (
                                `id` INT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '模块主键ID',
                                `module_code` VARCHAR(32) NOT NULL COMMENT '模块代号: LOBBY_SHELL (大厅外壳), ERRORS (错误码), FISHING (钓鱼机台), MINER (黄金矿工), PROMO (活动系统)',
                                `module_name` VARCHAR(64) NOT NULL COMMENT '模块业务中文描述',
                                `load_strategy` VARCHAR(16) NOT NULL DEFAULT 'LAZY' COMMENT '加载策略: EAGER (大厅启动必须预载), LAZY (进入该游戏时动态按需加载)',
                                `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                PRIMARY KEY (`id`),
                                UNIQUE KEY `uk_module_code` (`module_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='多语言模块命名空间与资源分包加载策略表';

-- -----------------------------------------------------------------------------
-- 3. 平台全局多语言基础词条表 (i18n_messages)
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `i18n_messages`;
CREATE TABLE `i18n_messages` (
                                 `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '词条自增主键ID',
                                 `module_code` VARCHAR(32) NOT NULL COMMENT '所属模块 (关联 i18n_modules.module_code)',
                                 `message_key` VARCHAR(128) NOT NULL COMMENT '程序引用的唯一键名 (如: btn_spin, err_insufficient_balance, fish_tuna_title)',
                                 `lang_code` VARCHAR(16) NOT NULL COMMENT '语种代码 (关联 i18n_languages.lang_code)',
                                 `message_text` TEXT NOT NULL COMMENT '翻译译文内容 (支持动态插值占位符，如: "余额不足，需至少 {min_bet} {currency}")',
                                 `context_remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '语境上下文说明 (用于辅助翻译及大模型校对)',
                                 `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 0-待翻译/草稿, 1-已校对生效, 2-已废弃',
                                 `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
                                 `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                 PRIMARY KEY (`id`),
    -- 核心唯一约束: 锁定 模块 + 键 + 语种
                                 UNIQUE KEY `uk_module_key_lang` (`module_code`, `message_key`, `lang_code`),
    -- 优化查询索引: 高频按语种批量导出、按模块加载语言包
                                 KEY `idx_lang_module` (`lang_code`, `module_code`, `status`),
    -- 优化按 Key 反向搜索时前缀索引长度，收敛 B+ 树物理体积
                                 KEY `idx_key_prefix` (`message_key`(64))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='平台标准多语言词条文案字典与高可用检索表';

-- -----------------------------------------------------------------------------
-- 4. 商户级多语言个性化覆盖表 (i18n_merchant_overrides)
-- 作用: 满足不同包网商对专业术语、品牌词的私有化定制需求
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `i18n_merchant_overrides`;
CREATE TABLE `i18n_merchant_overrides` (
                                           `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '覆盖规则主键ID',
                                           `merchant_id` BIGINT UNSIGNED NOT NULL COMMENT '关联商户ID (merchant_accounts.id)',
                                           `merchant_code` VARCHAR(32) NOT NULL COMMENT '商户唯一代号',
                                           `module_code` VARCHAR(32) NOT NULL COMMENT '模块代号',
                                           `message_key` VARCHAR(128) NOT NULL COMMENT '被重写的目标 Key (必须存在于 i18n_messages)',
                                           `lang_code` VARCHAR(16) NOT NULL COMMENT '重写的语种代码',
                                           `custom_text` TEXT NOT NULL COMMENT '该商户定制展示的特殊文本',
                                           `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否启用该重写: 0-停用(恢复默认), 1-启用覆盖',
                                           `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '配置时间',
                                           `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                           PRIMARY KEY (`id`),
                                           UNIQUE KEY `uk_merchant_key_lang` (`merchant_id`, `module_code`, `message_key`, `lang_code`),
                                           KEY `idx_merchant_lookup` (`merchant_code`, `lang_code`, `module_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户专属多语言文案白牌定制覆盖表';

-- -----------------------------------------------------------------------------
-- 5. 语言包编译与版本发布快照表 (i18n_release_versions)
-- 作用: 支撑前端 HTTP 缓存利用、CDN 静态化分发以及增量热更新检测
-- -----------------------------------------------------------------------------
DROP TABLE IF EXISTS `i18n_release_versions`;
CREATE TABLE `i18n_release_versions` (
                                         `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '发布流水主键ID',
                                         `release_version` VARCHAR(32) NOT NULL COMMENT '版本标识代号 (如: v1.0.26_build14 或 MD5前8位)',
                                         `merchant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属商户ID (0 表示平台通用基线包，>0 表示商户定制差分包)',
                                         `module_code` VARCHAR(32) NOT NULL COMMENT '模块代号 (或 ALL 表示全局大包)',
                                         `lang_code` VARCHAR(16) NOT NULL COMMENT '语种代码',
                                         `checksum_hash` VARCHAR(64) NOT NULL COMMENT '编译生成产物的 SHA256 / MD5 指纹 (供客户端比对校验)',
                                         `cdn_json_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '编译上传至 CDN/OSS 后的直接静态 JSON 文件下载地址',
                                         `entry_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '该包包含的有效词条总数量',
                                         `file_size_bytes` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'JSON 产物文件大小 (字节)',
                                         `is_active` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '是否为当前最新推荐版本: 0-历史旧版本, 1-当前最新生效版',
                                         `published_by` VARCHAR(64) NOT NULL DEFAULT 'SYSTEM' COMMENT '发布操作人员或系统构建流水线代号',
                                         `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '发布上线时间',
                                         PRIMARY KEY (`id`),
                                         KEY `idx_version_lookup` (`merchant_id`, `module_code`, `lang_code`, `is_active`),
                                         KEY `idx_release_ver` (`release_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='多语言静态包编译发布与客户端增量缓存版本管理表';

SET FOREIGN_KEY_CHECKS = 1;


