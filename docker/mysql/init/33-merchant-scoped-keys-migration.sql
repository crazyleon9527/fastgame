-- 33-merchant-scoped-keys-migration.sql
-- 给对账相关表补 merchant_id 前导唯一键，消除跨商户 roundId 撞车
--
-- 【问题】
--   round_id 由游戏客户端生成（Crypto.randomUUID）。原唯一键只约束 round_id：
--     pending_transactions  uk_round_id (round_id)
--     wallet_pending_ops    uk_round_op (round_id, op_type)
--     game_round_replay     （无唯一键）
--   多商户下两个商户的客户端完全可能生成同一个 roundId，此时：
--     - 第二个商户的写会被唯一键拒绝，或
--     - 查询/对账按 round_id 命中别人商户的记录（跨商户串数据）。
--   必须把 merchant_id 放到唯一键最前，让"唯一"的语义落在租户维度内。
--
-- 【为什么必须 NOT NULL】
--   MySQL 的唯一索引把 NULL 视为彼此不同：若 merchant_id 可为 NULL，
--   同一商户插入两行 NULL+同一 round_id 不会冲突，唯一键形同虚设。
--   因此先回填、再收紧为 NOT NULL DEFAULT 0（0 表示未能归属的孤儿行，
--   由后续排查；用 0 而不是拒绝写入，避免新商户尚未建档时写入失败）。
--
-- 【顺序】
--   1) 回填 merchant_id（按 merchant_code 关联 merchants）
--   2) 记录孤儿行数（便于运维判断影响面）
--   3) 收紧为 NOT NULL
--   4) 重建唯一键：merchant_id 在最前

USE fastgame;

SET NAMES utf8mb4;

-- ---------------------------------------------------------------- 1. 回填
UPDATE pending_transactions t
JOIN merchants m ON m.merchant_code COLLATE utf8mb4_unicode_ci = t.merchant_code COLLATE utf8mb4_unicode_ci
SET t.merchant_id = m.id
WHERE t.merchant_id IS NULL;

UPDATE wallet_pending_ops w
JOIN merchants m ON m.merchant_code COLLATE utf8mb4_unicode_ci = w.merchant_code COLLATE utf8mb4_unicode_ci
SET w.merchant_id = m.id
WHERE w.merchant_id IS NULL;

UPDATE game_round_replay r
JOIN merchants m ON m.merchant_code COLLATE utf8mb4_unicode_ci = r.merchant_code COLLATE utf8mb4_unicode_ci
SET r.merchant_id = m.id
WHERE r.merchant_id IS NULL;

-- ---------------------------------------------------------------- 2. 孤儿行兜底
-- merchant_code 在 merchants 里找不到的行（历史脏数据 / 测试数据）置 0，
-- 避免后面的 NOT NULL 收紧失败。运维可用下面的 SELECT 复核影响面：
--   SELECT 'pending_transactions', COUNT(*) FROM pending_transactions WHERE merchant_id IS NULL;
UPDATE pending_transactions SET merchant_id = 0 WHERE merchant_id IS NULL;
UPDATE wallet_pending_ops    SET merchant_id = 0 WHERE merchant_id IS NULL;
UPDATE game_round_replay     SET merchant_id = 0 WHERE merchant_id IS NULL;

-- ---------------------------------------------------------------- 3. 收紧非空
ALTER TABLE pending_transactions
  MODIFY COLUMN merchant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '商户 ID（merchants.id），0=未归属';

ALTER TABLE wallet_pending_ops
  MODIFY COLUMN merchant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '商户 ID（merchants.id），0=未归属';

ALTER TABLE game_round_replay
  MODIFY COLUMN merchant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '商户 ID（merchants.id），0=未归属';

-- ---------------------------------------------------------------- 4. 重建唯一键
-- 先建新键再删旧键：任一步失败都不会出现"完全没有唯一约束"的窗口。
ALTER TABLE pending_transactions
  ADD UNIQUE KEY uk_merchant_round (merchant_id, round_id),
  DROP INDEX uk_round_id;

ALTER TABLE wallet_pending_ops
  ADD UNIQUE KEY uk_merchant_round_op (merchant_id, round_id, op_type),
  DROP INDEX uk_round_op;

-- game_round_replay：旧键是 05-replay-migration.sql 里定义的 uk_round_id(round_id)，
-- 它同样只按 round_id 判重，是跨商户撞车的直接原因，必须一并移除。
-- 先加新键再删旧键，避免出现"完全没有唯一约束"的窗口。
ALTER TABLE game_round_replay
  ADD UNIQUE KEY uk_merchant_round (merchant_id, round_id),
  DROP INDEX uk_round_id;

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('33-merchant-scoped-keys', 'Scope unique keys by merchant_id on reconciliation tables');

SELECT '33-merchant-scoped-keys-migration applied' AS note;
