-- 30-translate-comments-rollback.sql
-- 回滚：把表/列注释恢复为翻译前的英文。
-- 由 scripts/gen_comment_translate.ps1 生成。
USE fastgame;

SET NAMES utf8mb4;

-- 表注释
ALTER TABLE `risk_alerts` COMMENT = '';

DELETE FROM schema_migrations WHERE version = '30-translate-comments';
