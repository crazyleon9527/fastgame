-- 32-outbox-migration.sql
-- Transactional Outbox：事件发件箱 + 消费端 DB 兜底幂等
--
-- 目的：进程（RGS/DLQ 出口）不再直接向 Kafka 投递。业务与"事件已登记"在同一事务提交，
-- 由 outbox dispatcher 从表里领取并投递，投递失败保留 pending 重试，
-- 从而消除"扣了钱但事件丢了"（原实现是裸 go func + RequiredAcks 未设 +
-- 消费端先 commit offset 再落库）。
--
-- 参考 platform-api `internal/app/outbox`（已过市场验证）的设计，并按本项目
-- 的 go-zero + sqlx 技术栈重写，不引入 GORM。

USE fastgame;

SET NAMES utf8mb4;

-- 事件发件箱
CREATE TABLE IF NOT EXISTS event_outbox (
  id             CHAR(36)     NOT NULL COMMENT '事件 ID：UUID v7（36 字符，含连字符）',
  topic          VARCHAR(64)  NOT NULL COMMENT 'Kafka topic',
  merchant_id    BIGINT UNSIGNED NULL COMMENT '商户 ID（merchants.id），便于多租户与排查',
  partition_key  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'Kafka 分区键，取 user_id 保证同玩家时序',
  payload        JSON         NOT NULL COMMENT '事件信封 JSON（含 eventId/traceId/trace/业务字段）',
  status         TINYINT      NOT NULL DEFAULT 0 COMMENT '0=PENDING 1=IN_FLIGHT 2=SENT 3=FAILED',
  delivery_mode  TINYINT      NOT NULL DEFAULT 1 COMMENT '1=kafka（保留位：2=pubsub 3=both）',
  retry_count    INT          NOT NULL DEFAULT 0 COMMENT '已投递失败次数',
  next_retry_at  DATETIME(3)  NOT NULL COMMENT '下次可投递时间（退避）',
  claim_owner    VARCHAR(64)  NULL COMMENT '领取者标识（instance:goroutine），SettleBatch 校验用',
  claim_at       DATETIME(3)  NULL COMMENT '领取时间，Reaper 据此回收悬挂 IN_FLIGHT',
  sent_at        DATETIME(3)  NULL COMMENT '投递成功时间，GC 据此清理',
  last_error     VARCHAR(512) NULL COMMENT '最近一次投递错误',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  -- 三条索引对应三种扫描模式：派发、领取、清理（与 platform-api 一致）
  KEY idx_dispatch (status, next_retry_at, id),
  KEY idx_claim (status, claim_at),
  KEY idx_sent_at (sent_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='事件发件箱（事务性消息投递）';

-- 消费端 DB 兜底幂等表
--
-- 与 Redis SETNX 形成双层防重：Redis 兜住常规重投，DB 兜住 Redis 重启/键被清理。
-- 关键：占位行必须与业务写在**同一个事务**内提交（AcquireInTx），
-- 否则进程在"占位已写、业务未完成"之间崩溃会留下幽灵占位行，
-- 使下次重投被错误 ACK、事件永久丢失。
CREATE TABLE IF NOT EXISTS processed_events (
  consumer_group VARCHAR(64) NOT NULL COMMENT '消费组名（事件类型维度）',
  event_id       CHAR(36)    NOT NULL COMMENT '事件 ID：UUID v7（36 字符）',
  processed_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '占位时间，供 GC 清理',
  PRIMARY KEY (consumer_group, event_id),
  KEY idx_processed_at (processed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='消费端事件幂等占位（与业务同事务写入）';

INSERT IGNORE INTO schema_migrations (version, description)
VALUES ('32-outbox', 'Transactional outbox (event_outbox) + consumer dedup (processed_events)');

SELECT '32-outbox-migration applied' AS note;
