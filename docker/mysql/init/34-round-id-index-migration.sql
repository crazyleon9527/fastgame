-- 34-round-id-index-migration.sql
-- 为「只按 round_id 查询」的只读路径补回单列索引
--
-- 【背景】
--   33-merchant-scoped-keys 把三张对账表（pending_transactions /
--   wallet_pending_ops / game_round_replay）的唯一键改成了 merchant_id 前导：
--     uk_merchant_round(merchant_id, round_id)
--     uk_merchant_round_op(merchant_id, round_id, op_type)
--   同时 DROP 掉了原来的 uk_round_id(round_id)。索引一旦变成复合键，
--   `WHERE round_id = ?` 就用不上它（round_id 不是最左前缀），退化成全表扫描。
--
-- 【谁还会只按 round_id 查】
--   写路径（下注结算的 MarkSettled / MarkWinPending）已改成 (merchant_id, round_id)
--   定位，走 uk_merchant_round。剩下两类只读路径拿不到商户上下文，只能按 round_id 查：
--     - 公开回放 GET /api/v1/game/replay/:roundId（客户端不传 merchantId 时）
--     - 后台排障 GET /admin/.../traces/round/:roundId（不传 merchantId 时）
--   这两处若不补索引，每次请求都会全表扫描 game_round_replay /
--   pending_transactions——而这两张表随每笔下注增长。
--
-- 【为什么是普通索引而不是唯一索引】
--   round_id 现在只在商户内唯一，不同商户可以合法复用同一个 roundId，
--   所以这里只能是普通索引，绝不能恢复唯一约束（否则 33 的修复就被推翻了）。
--
-- 幂等：用 information_schema 判断后再动态执行，可重复运行。

USE fastgame;

SET NAMES utf8mb4;

-- MySQL 8.0 没有 ADD INDEX IF NOT EXISTS，用动态 SQL 做条件执行
-- （不用存储过程，避免依赖 DELIMITER）。

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.STATISTICS
         WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pending_transactions'
           AND INDEX_NAME = 'idx_round_id'),
  'SELECT ''pending_transactions.idx_round_id already exists'' AS note',
  'ALTER TABLE `pending_transactions` ADD INDEX `idx_round_id` (`round_id`)'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.STATISTICS
         WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'game_round_replay'
           AND INDEX_NAME = 'idx_round_id'),
  'SELECT ''game_round_replay.idx_round_id already exists'' AS note',
  'ALTER TABLE `game_round_replay` ADD INDEX `idx_round_id` (`round_id`)'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.STATISTICS
         WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'wallet_pending_ops'
           AND INDEX_NAME = 'idx_round_id'),
  'SELECT ''wallet_pending_ops.idx_round_id already exists'' AS note',
  'ALTER TABLE `wallet_pending_ops` ADD INDEX `idx_round_id` (`round_id`)'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('34-round-id-index', 'Add single-column round_id indexes for merchant-unscoped read paths');

SELECT '34-round-id-index-migration applied' AS note;
