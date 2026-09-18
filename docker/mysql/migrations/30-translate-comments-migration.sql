-- 30-translate-comments-migration.sql
-- 把英文的表/列注释改为简体中文（枚举值与专有名词保留英文）。
-- 译法定义在 docker/mysql/migrations/translations_comments.json。
-- 列定义由 information_schema 重建，只替换 COMMENT，类型/默认值/extra 不变。
-- 由 scripts/gen_comment_translate.ps1 生成，请勿手工编辑。
USE fastgame;

SET NAMES utf8mb4;

-- 表注释
ALTER TABLE `risk_alerts` COMMENT = 'RTP 风控告警';

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('30-translate-comments', 'Translate English table/column comments into Simplified Chinese');

SELECT '30-translate-comments applied' AS note;
