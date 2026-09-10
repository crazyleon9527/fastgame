-- FastGame MySQL 初始化脚本
-- 字符集 utf8mb4，时区 UTC（由 MySQL 容器 command 参数保证）

SET NAMES utf8mb4;
SET time_zone = '+00:00';

CREATE DATABASE IF NOT EXISTS fastgame
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE fastgame;

-- 商户主体
CREATE TABLE IF NOT EXISTS merchants (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_code VARCHAR(64)     NOT NULL COMMENT '商户唯一编码',
  name          VARCHAR(128)    NOT NULL COMMENT '商户名称',
  public_key    TEXT            NULL     COMMENT 'API 公钥',
  private_key   TEXT            NULL     COMMENT 'API 私钥 (加密存储)',
  private_key_prev TEXT         NULL     COMMENT '轮换过渡期旧私钥',
  private_key_prev_expires_at DATETIME(3) NULL COMMENT '旧私钥失效时间',
  status        TINYINT         NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_code (merchant_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商户主体';

-- 游戏参数字典 (多档 RTP 等)
CREATE TABLE IF NOT EXISTS game_configs (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id  BIGINT UNSIGNED NOT NULL,
  game_code    VARCHAR(64)     NOT NULL COMMENT '游戏标识',
  config_key   VARCHAR(128)    NOT NULL COMMENT '配置键',
  config_value JSON            NOT NULL COMMENT '配置值',
  rtp_tier     VARCHAR(32)     NULL     COMMENT 'RTP 档位',
  status       TINYINT         NOT NULL DEFAULT 1,
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_game_key (merchant_id, game_code, config_key),
  KEY idx_merchant_game (merchant_id, game_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='游戏参数配置';

-- RBAC 角色
CREATE TABLE IF NOT EXISTS roles (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name        VARCHAR(64)     NOT NULL,
  description VARCHAR(256)    NULL,
  created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_role_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色';

-- RBAC 用户
CREATE TABLE IF NOT EXISTS admin_users (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username      VARCHAR(64)     NOT NULL,
  password_hash VARCHAR(256)    NOT NULL,
  role_id       BIGINT UNSIGNED NOT NULL,
  status        TINYINT         NOT NULL DEFAULT 1,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_username (username),
  KEY idx_role_id (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理后台用户';

-- 每日结算对账单
CREATE TABLE IF NOT EXISTS daily_settlements (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  merchant_id  BIGINT UNSIGNED NOT NULL,
  settle_date  DATE            NOT NULL COMMENT '结算日期 UTC',
  total_bet    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  total_win    DECIMAL(20, 4)  NOT NULL DEFAULT 0,
  total_rounds BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status       TINYINT         NOT NULL DEFAULT 0 COMMENT '0=待确认 1=已确认',
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_merchant_date (merchant_id, settle_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='每日结算对账单';

-- 风控黑名单 (IP / 用户 / 商户)
CREATE TABLE IF NOT EXISTS risk_blacklist (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  list_type    VARCHAR(32)     NOT NULL COMMENT 'ip / user_id / merchant',
  list_value   VARCHAR(128)    NOT NULL COMMENT '黑名单值',
  reason       VARCHAR(256)    NULL     COMMENT '封禁原因',
  status       TINYINT         NOT NULL DEFAULT 1 COMMENT '1=生效 0=解除',
  expires_at   DATETIME(3)     NULL     COMMENT 'NULL=永久',
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_type_value (list_type, list_value),
  KEY idx_status_expires (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='风控黑名单';
