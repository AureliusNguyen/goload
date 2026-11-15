CREATE TABLE
    tasks (
        id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
        of_account_id BIGINT UNSIGNED NOT NULL,
        file_name TEXT NOT NULL,
        source_url TEXT NOT NULL,
        source_type VARCHAR(32) NOT NULL,
        headers JSON,
        source_auth JSON,
        storage_type VARCHAR(32) NOT NULL,
        storage_path TEXT NOT NULL,
        -- Checksum verification
        checksum_type TEXT, -- "md5", "sha1", "sha256"
        checksum_value TEXT,
        -- Download behavior
        concurrency INT DEFAULT 4,
        max_speed BIGINT, -- bytes/sec
        max_retries INT NOT NULL DEFAULT 3,
        timeout INT, -- seconds
        status VARCHAR(32) NOT NULL,
        progress FLOAT DEFAULT 0.00, -- percent, e.g., 45.67
        downloaded_bytes BIGINT DEFAULT 0,
        total_bytes BIGINT DEFAULT 0,
        error_message TEXT,
        metadata JSON,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        completed_at DATETIME,
        last_accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        expiration_days INT UNSIGNED DEFAULT 30, -- days
        INDEX (of_account_id),
        INDEX (status)
    );

CREATE TABLE
    task_outbox (
        id CHAR(36) NOT NULL,
        aggregate_id BIGINT UNSIGNED NOT NULL,
        destination VARCHAR(32) NOT NULL,
        event_type VARCHAR(64) NOT NULL,
        payload MEDIUMBLOB NOT NULL,
        created_at DATETIME(6) NOT NULL,
        sent_at DATETIME(6) NULL DEFAULT NULL,
        PRIMARY KEY (id),
        INDEX task_outbox_unsent_idx (sent_at, created_at),
        INDEX task_outbox_aggregate_idx (aggregate_id)
    );