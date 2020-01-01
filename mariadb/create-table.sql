CREATE TABLE IF NOT EXISTS `hostdb` (
    `id`        char(64)     NOT NULL CHECK (`id` <> ''),
    `type`      varchar(128) NOT NULL CHECK (`type` <> ''),
    `hostname`  varchar(256) NOT NULL,
    `ip`        varchar(45)  NOT NULL,
    `timestamp` timestamp    NOT NULL DEFAULT current_timestamp(),
    `committer` varchar(256) NOT NULL CHECK (`committer` <> ''),
    `context`   longtext     NOT NULL CHECK (json_valid(`context`)),
    `data`      longtext     NOT NULL CHECK (json_valid(`data`)),
    `hash`      varchar(64)  NOT NULL CHECK (`hash` <> ''),
    PRIMARY KEY (`id`),
    UNIQUE KEY `id` (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8 COMMENT ='HostDB records'