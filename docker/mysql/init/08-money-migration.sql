-- 08-money-migration.sql
-- 历史 DECIMAL 金额列迁移至 int64 minor units (Scale=10000) 的参考脚本。
-- 新表/新列已直接使用 BIGINT minor；本脚本供存量库一次性升级时参考。

-- 示例：game_rounds 若仍存 DECIMAL(18,4) major，可转为 minor：
-- UPDATE game_rounds SET bet_amount_minor = ROUND(bet_amount * 10000), win_amount_minor = ROUND(win_amount * 10000);
-- ALTER TABLE game_rounds DROP COLUMN bet_amount, DROP COLUMN win_amount;
-- ALTER TABLE game_rounds CHANGE bet_amount_minor bet_amount BIGINT NOT NULL;
-- ALTER TABLE game_rounds CHANGE win_amount_minor win_amount BIGINT NOT NULL;

-- pending_transactions / wallet 对账表已在 06-orphan-trace-migration 使用 BIGINT minor。
-- API/Kafka/Wallet 协议字段 betAmount/winAmount 均为 int64 minor，客户端展示除以 10000。

SELECT '08-money-migration: reference only — apply manually if legacy DECIMAL columns exist' AS note;
