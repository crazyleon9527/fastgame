-- 29-repair-column-comments-rollback.sql
-- 回滚：把表/列注释恢复为修复前的值（HEX 确认过的原始损坏内容）。
-- 由 scripts/gen_comment_repair.ps1 生成。
USE fastgame;

SET NAMES utf8mb4;

-- 表注释
ALTER TABLE `pending_transactions` COMMENT = 'å­¤å„¿æ³¨å•è‡ªåŠ¨å¯¹è´¦è¡¥å¿';

DELETE FROM schema_migrations WHERE version = '29-repair-column-comments';
