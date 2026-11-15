-- name: CreateTask :execresult
INSERT INTO tasks (of_account_id, file_name, source_url, source_type, source_auth, headers,
                   storage_type, storage_path, status,
                   checksum_type, checksum_value,
                   concurrency, max_speed, max_retries, timeout, metadata, expiration_days)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetTaskById :one
SELECT *
FROM tasks
WHERE id = ?;

-- name: UpdateTaskStatus :exec
UPDATE tasks
SET status = ?
WHERE id = ?;

-- name: UpdateTaskCompletedAt :exec
UPDATE tasks
SET completed_at = ?
WHERE id = ?;

-- name: UpdateFileName :exec
UPDATE tasks
SET file_name = ?
WHERE id = ?;

-- name: UpdateTaskError :exec
UPDATE tasks
SET error_message = ?
WHERE id = ?;

-- name: UpdateTaskDownloadedBytes :exec
UPDATE tasks
SET downloaded_bytes = ?
WHERE id = ?;

-- name: UpdateTaskTotalBytes :exec
UPDATE tasks
SET total_bytes = ?
WHERE id = ?;

-- name: UpdateTaskMetadata :exec
UPDATE tasks
SET metadata = ?
WHERE id = ?;

-- name: UpdateTaskProgress :exec
UPDATE tasks
SET progress = ?
WHERE id = ?;

-- name: UpdateStorageInfo :exec
UPDATE tasks
SET storage_type = ?, storage_path = ?
WHERE id = ?;

-- name: UpdateFileChecksum :exec
UPDATE tasks
SET checksum_type = ?, checksum_value = ?
WHERE id = ?;

-- name: ListTasks :many
SELECT *
FROM tasks
WHERE of_account_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetTaskCountByAccountId :one
SELECT COUNT(*)
FROM tasks
WHERE of_account_id = ?;

-- name: DeleteTask :exec
DELETE
FROM tasks
WHERE id = ?;

-- name: InsertOutbox :execresult
INSERT INTO task_outbox (id, aggregate_id, destination, event_type, payload, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: FetchUnsentOutbox :many
SELECT *
FROM task_outbox
WHERE sent_at IS NULL
ORDER BY created_at ASC, id ASC
LIMIT ?;

-- name: FetchUnsentOutboxAfter :many
SELECT *
FROM task_outbox
WHERE sent_at IS NULL AND (created_at > ? OR (created_at = ? AND id > ?))
ORDER BY created_at ASC, id ASC
LIMIT ?;

-- name: MarkOutboxSent :exec
UPDATE task_outbox
SET sent_at = ?
WHERE id = ? AND sent_at IS NULL;
