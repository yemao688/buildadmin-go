ALTER TABLE {{table}} ADD COLUMN `legacy_unrecoverable` tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '历史目标管理员不可恢复';
