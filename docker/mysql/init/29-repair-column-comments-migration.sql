-- 29-repair-column-comments-migration.sql
-- 修复被错误 charset 毁掉的表/列注释（ASCII 问号 / 双重编码乱码）。
-- 注释内容取自 docker/mysql/migrations/comment_manifest.json（由迁移文件提取）。
-- 列定义由 information_schema 重建，只替换 COMMENT，类型/默认值/extra 不变。
-- 由 scripts/gen_comment_repair.ps1 生成，请勿手工编辑。
USE fastgame;

SET NAMES utf8mb4;

-- 表注释
ALTER TABLE `pending_transactions` COMMENT = '孤儿注单自动对账补偿';

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('29-repair-column-comments', 'Restore Chinese column comments damaged by wrong charset');

SELECT '29-repair-column-comments applied' AS note;
