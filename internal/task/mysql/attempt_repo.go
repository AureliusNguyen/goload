package mysql

import (
	"context"
	"database/sql"
	stderrors "errors"
	"time"

	"github.com/yuisofull/goload/internal/errors"
	"github.com/yuisofull/goload/internal/storage"
	task "github.com/yuisofull/goload/internal/task"
)

// errAttemptNeedsTx is returned when the lock or the save is attempted
// without the transaction DoInTx stores in ctx.
var errAttemptNeedsTx = stderrors.New("mysql: attempt state must be locked and saved inside TxManager.DoInTx")

const selectAttemptStateSQL = `SELECT status, owner_id, lease_expires_at, attempt_generation, retry_count, next_attempt_at,
       max_retries, file_name, storage_type, storage_path, checksum_type, checksum_value,
       progress, downloaded_bytes, total_bytes, error_message, completed_at
FROM tasks
WHERE id = ?`

// lockAttemptStateSQL keeps the row locked until the transaction ends, so
// concurrent transitions of one task run one after another.
const lockAttemptStateSQL = selectAttemptStateSQL + `
FOR UPDATE`

const saveAttemptStateSQL = `UPDATE tasks
SET status = ?, owner_id = ?, lease_expires_at = ?, attempt_generation = ?, retry_count = ?, next_attempt_at = ?,
    file_name = ?, storage_type = ?, storage_path = ?, checksum_type = ?, checksum_value = ?,
    progress = ?, downloaded_bytes = ?, total_bytes = ?, error_message = ?, completed_at = ?
WHERE id = ?`

type attemptRepo struct {
	db *sql.DB
}

// NewAttemptRepo returns the MySQL implementation of task.AttemptRepository.
// It uses plain database/sql (no sqlc) and needs parseTime=true in the DSN,
// which cmd/task already sets.
func NewAttemptRepo(db *sql.DB) task.AttemptRepository {
	return &attemptRepo{db: db}
}

func (r *attemptRepo) GetAttemptState(ctx context.Context, taskID uint64) (*task.AttemptState, error) {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return scanAttemptState(tx.QueryRowContext(ctx, selectAttemptStateSQL, taskID), taskID)
	}
	return scanAttemptState(r.db.QueryRowContext(ctx, selectAttemptStateSQL, taskID), taskID)
}

func (r *attemptRepo) LockAttemptState(ctx context.Context, taskID uint64) (*task.AttemptState, error) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	if !ok {
		return nil, errAttemptNeedsTx
	}
	return scanAttemptState(tx.QueryRowContext(ctx, lockAttemptStateSQL, taskID), taskID)
}

func (r *attemptRepo) SaveAttemptState(ctx context.Context, state *task.AttemptState) error {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	if !ok {
		return errAttemptNeedsTx
	}

	var checksumType, checksumValue, errorMessage sql.NullString
	if state.Checksum != nil {
		checksumType = sql.NullString{String: state.Checksum.ChecksumType, Valid: true}
		checksumValue = sql.NullString{String: state.Checksum.ChecksumValue, Valid: true}
	}
	if state.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *state.ErrorMessage, Valid: true}
	}

	_, err := tx.ExecContext(ctx, saveAttemptStateSQL,
		string(state.Status),
		sql.NullString{String: state.OwnerID, Valid: state.OwnerID != ""},
		nullTime(state.LeaseExpiresAt, time.Microsecond),
		state.Generation,
		state.RetryCount,
		nullTime(state.NextAttemptAt, time.Microsecond),
		state.FileName,
		string(state.StorageType),
		state.StoragePath,
		checksumType,
		checksumValue,
		state.Progress.Progress,
		state.Progress.DownloadedBytes,
		state.Progress.TotalBytes,
		errorMessage,
		nullTime(state.CompletedAt, time.Second),
		state.TaskID,
	)
	return err
}

// nullTime binds a nil time as NULL. Other times are truncated to the
// precision the column stores (DATETIME(6): microseconds, DATETIME: seconds),
// because MySQL rounds a longer fraction and could round up.
func nullTime(t *time.Time, precision time.Duration) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC().Truncate(precision), Valid: true}
}

// scanAttemptState maps one selectAttemptStateSQL row.
func scanAttemptState(row *sql.Row, taskID uint64) (*task.AttemptState, error) {
	var (
		status        string
		ownerID       sql.NullString
		leaseExpires  sql.NullTime
		generation    uint64
		retryCount    int
		nextAttemptAt sql.NullTime
		maxRetries    int
		fileName      string
		storageType   string
		storagePath   string
		checksumType  sql.NullString
		checksumValue sql.NullString
		progress      sql.NullFloat64
		downloaded    sql.NullInt64
		total         sql.NullInt64
		errorMessage  sql.NullString
		completedAt   sql.NullTime
	)
	err := row.Scan(
		&status, &ownerID, &leaseExpires, &generation, &retryCount, &nextAttemptAt,
		&maxRetries, &fileName, &storageType, &storagePath, &checksumType, &checksumValue,
		&progress, &downloaded, &total, &errorMessage, &completedAt,
	)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, err
	}

	state := &task.AttemptState{
		TaskID:     taskID,
		Status:     task.TaskStatus(status),
		OwnerID:    ownerID.String, // NULL reads as ""
		Generation: generation,
		RetryCount: retryCount,
		MaxRetries: maxRetries,
		FileName:   fileName,
		// Kept exactly as stored (no normalization), so a save writes the
		// same value back.
		StorageType: storage.Type(storageType),
		StoragePath: storagePath,
		Progress: task.DownloadProgress{
			Progress:        progress.Float64,
			DownloadedBytes: downloaded.Int64,
			TotalBytes:      total.Int64,
		},
	}
	if leaseExpires.Valid {
		state.LeaseExpiresAt = &leaseExpires.Time
	}
	if nextAttemptAt.Valid {
		state.NextAttemptAt = &nextAttemptAt.Time
	}
	if checksumType.Valid || checksumValue.Valid {
		state.Checksum = &task.ChecksumInfo{
			ChecksumType:  checksumType.String,
			ChecksumValue: checksumValue.String,
		}
	}
	if errorMessage.Valid {
		state.ErrorMessage = &errorMessage.String
	}
	if completedAt.Valid {
		state.CompletedAt = &completedAt.Time
	}
	return state, nil
}
