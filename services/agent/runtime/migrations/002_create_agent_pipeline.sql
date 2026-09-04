CREATE TABLE IF NOT EXISTS agent_bots (
    id BIGINT NOT NULL AUTO_INCREMENT,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) NULL,
    avatar_url VARCHAR(500) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_bot_code_deleted (code, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS room_agent_bindings (
    id BIGINT NOT NULL AUTO_INCREMENT,
    room_id BIGINT NOT NULL,
    bot_id BIGINT NOT NULL,
    legacy_bot_user_id BIGINT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 100,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_room_bot_deleted (room_id, bot_id, deleted_at),
    KEY idx_room_agent_enabled (room_id, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_message_inbox (
    id BIGINT NOT NULL AUTO_INCREMENT,
    event_id VARCHAR(160) NOT NULL,
    topic VARCHAR(100) NOT NULL,
    partition_id INT NOT NULL,
    kafka_offset BIGINT NOT NULL,
    room_id BIGINT NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    sender_id BIGINT NOT NULL,
    sender_type VARCHAR(20) NOT NULL DEFAULT 'user',
    content TEXT NOT NULL,
    message_time DATETIME NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_inbox_event (event_id),
    UNIQUE KEY uk_agent_kafka_position (topic, partition_id, kafka_offset),
    KEY idx_agent_inbox_pending (status, room_id, message_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS moderation_records (
    id BIGINT NOT NULL AUTO_INCREMENT,
    inbox_id BIGINT NOT NULL,
    room_id BIGINT NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    sender_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    rule_code VARCHAR(50) NULL,
    reason TEXT NULL,
    evidence TEXT NULL,
    confidence DOUBLE NULL,
    raw_result JSON NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_moderation_inbox (inbox_id),
    KEY idx_moderation_room_sender (room_id, sender_id),
    KEY idx_moderation_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS removal_requests (
    id BIGINT NOT NULL AUTO_INCREMENT,
    room_id BIGINT NOT NULL,
    target_user_id BIGINT NOT NULL,
    moderation_record_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    evidence JSON NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewed_by_user_id BIGINT NULL,
    reviewed_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_removal_room_status (room_id, status),
    KEY idx_removal_target_user (target_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS agent_chunks (
    id BIGINT NOT NULL AUTO_INCREMENT,
    room_id BIGINT NOT NULL,
    binding_id BIGINT NOT NULL,
    topic_name VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    message_ids JSON NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    qdrant_point_id VARCHAR(64) NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_agent_chunks_qdrant_point (qdrant_point_id),
    KEY idx_agent_chunks_room_time (room_id, start_time, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
