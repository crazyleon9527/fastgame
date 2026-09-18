-- MySQL portion extracted from report.sql (ClickHouse DDL lives in clickhouse.sql)
SET NAMES utf8mb4;

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
