ALTER TABLE {{table}} ADD COLUMN `is_committed` tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '提交状态';
