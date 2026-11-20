# Reconcile index (reliable dispatch, step 2b).
# The periodic reconciler looks for tasks that should be running but have no
# live attempt: PENDING tasks that are unowned and due, and DOWNLOADING or
# STORING tasks whose lease expired. It reads tasks by status and lease
# expiry; the status prefix lets it skip the finished tasks.
#
# Idempotent on purpose: run-migrations.sh applies every file on each start,
# and MySQL has no CREATE INDEX IF NOT EXISTS. The index is added through a
# prepared statement only when information_schema does not list it yet.

SET @exist := (
    SELECT COUNT(*)
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'tasks'
      AND index_name = 'tasks_reconcile_idx'
);
SET @sqlstmt := IF(
    @exist = 0,
    'ALTER TABLE tasks ADD INDEX tasks_reconcile_idx (status, lease_expires_at)',
    'SELECT 1'
);
PREPARE stmt FROM @sqlstmt;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
