CREATE TABLE IF NOT EXISTS {{closure}} (
    `ancestor_id` int(11) unsigned NOT NULL,
    `descendant_id` int(11) unsigned NOT NULL,
    `depth` int(11) unsigned NOT NULL DEFAULT 0,
    PRIMARY KEY (`ancestor_id`,`descendant_id`),
    KEY `idx_descendant_ancestor` (`descendant_id`,`ancestor_id`),
    KEY `idx_ancestor_depth` (`ancestor_id`,`depth`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS {{lock}} (
    `id` tinyint(3) unsigned NOT NULL,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
