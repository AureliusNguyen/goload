# Task result identity (reliable dispatch, PR2b).
# A completed attempt persists the identity of its result so a completion
# retry that reports a different result can be rejected instead of being
# treated as the idempotent lost-response retry.
#
# result_content_type  content type of the committed result; '' when unset
# result_file_size     byte size of the committed result; 0 for an empty file
#
# These are distinct from total_bytes and downloaded_bytes, which are live
# progress. The size is stored unconditionally so a faithful retry of an
# empty file still matches.
#
# Idempotent on purpose: run-migrations.sh applies every file on each start,
# and MySQL has no ADD COLUMN IF NOT EXISTS. Each column is added through a
# prepared statement only when information_schema does not list it yet.

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND column_name = 'result_content_type'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN result_content_type VARCHAR(255) NOT NULL DEFAULT ''''',
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
      AND column_name = 'result_file_size'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD COLUMN result_file_size BIGINT NOT NULL DEFAULT 0',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
