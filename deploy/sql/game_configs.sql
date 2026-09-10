CREATE TABLE `game_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `merchant_id` bigint unsigned NOT NULL,
  `game_code` varchar(64) NOT NULL,
  `config_key` varchar(128) NOT NULL,
  `config_value` json NOT NULL,
  `rtp_tier` varchar(32) DEFAULT NULL,
  `status` tinyint NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_merchant_game_key` (`merchant_id`,`game_code`,`config_key`),
  KEY `idx_merchant_game` (`merchant_id`,`game_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
