# Task ownership state (reliable dispatch, step 2).
# A download worker claims a task before it executes it. The claim lives on
# the task row and every later write by that attempt is checked against it.
#
# owner_id            worker currently holding the task; NULL means unowned
# lease_expires_at    when the current claim expires unless it is renewed
# attempt_generation  fencing token; starts at 0, plus one on every claim
# retry_count         number of failed attempts so far
# next_attempt_at     earliest time a retry may be claimed; NULL means now
#
# Idempotent on purpose: run-migrations.sh applies every file on each start,
# and MySQL has no ADD COLUMN IF NOT EXISTS. Each column is added through a
# prepared statement only when information_schema does not list it yet.

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND column_name = 'owner_id'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN owner_id VARCHAR(64) NULL DEFAULT NULL',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND column_name = 'lease_expires_at'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN lease_expires_at DATETIME(6) NULL DEFAULT NULL',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND column_name = 'attempt_generation'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN attempt_generation BIGINT UNSIGNED NOT NULL DEFAULT 0',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND column_name = 'retry_count'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN retry_count INT UNSIGNED NOT NULL DEFAULT 0',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND column_name = 'next_attempt_at'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN next_attempt_at DATETIME(6) NULL DEFAULT NULL',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
