-- 36-ledger-idempotency-by-status.sql
-- 账变幂等键加入终态：同一局同一类型允许"一次尝试 + 一次成功落账"共存
--
-- 【问题】
--   35 号迁移的幂等键是 uk_merchant_round_type(merchant_id, round_id, tx_type)。
--   但一笔钱可能经历多个终态，最典型的是派彩超时后补偿：
--     WIN  PENDING_RETRY  +500  → 钱没动（等补偿）
--     WIN  SUCCESS        +500  → 补偿完成，钱真的到账
--   这两条都是必须留下的事实。原键会把第二条判成重复而丢弃，
--   结果是"钱到了但账上没有"——比没有账变更糟。
--
-- 【改法】
--   幂等键加入 status：唯一性落在"同一局的同一类型，每种终态只记一次"。
--     · 重放同一次投递（同终态）→ 命中唯一键，被幂等挡住，不重复扣钱；
--     · 首次尝试失败 + 后续补偿成功 → 两条并存，各自如实记录；
--     · 人工调账 round_id 为 NULL，MySQL 唯一索引视 NULL 互不相同，不受约束。
--
-- 【为什么按"终态"而不是给补偿单开一个类型】
--   补偿在业务上就是同一笔派彩，另立类型会让"总派彩"必须同时统计两个 code，
--   任何漏加都会算错账。按终态区分后，对 SUCCESS 行求和就是真实资金流。

USE fastgame;

SET NAMES utf8mb4;

-- MySQL 8.0 不支持 ALTER 里 IF EXISTS 判断索引，用动态 SQL 条件执行（可重放）
SET @has_old = (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'game_transactions'
    AND INDEX_NAME = 'uk_merchant_round_type'
);
SET @has_new = (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'game_transactions'
    AND INDEX_NAME = 'uk_merchant_round_type_status'
);

SET @ddl = IF(@has_new > 0,
  'SELECT ''uk_merchant_round_type_status already exists'' AS note',
  IF(@has_old > 0,
     'ALTER TABLE `game_transactions` DROP INDEX `uk_merchant_round_type`, ADD UNIQUE KEY `uk_merchant_round_type_status` (`merchant_id`, `round_id`, `tx_type`, `status`)',
     'ALTER TABLE `game_transactions` ADD UNIQUE KEY `uk_merchant_round_type_status` (`merchant_id`, `round_id`, `tx_type`, `status`)'
  )
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE game_transactions
SET remark = remark
WHERE 1 = 0; -- 占位：保持本迁移只做结构变更，不改数据

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('36-ledger-idempotency-by-status', 'Include terminal status in ledger idempotency key so compensation entries can coexist');

SELECT '36-ledger-idempotency-by-status-migration applied' AS note;
