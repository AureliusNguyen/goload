package mysql

import (
	"context"
	"database/sql"
	"time"

	task "github.com/yuisofull/goload/internal/task"
)

const insertOutboxSQL = `INSERT INTO task_outbox (id, aggregate_id, destination, event_type, payload, created_at)
VALUES (?, ?, ?, ?, ?, ?)`

const fetchUnsentOutboxSQL = `SELECT id, aggregate_id, destination, event_type, payload, created_at
FROM task_outbox
WHERE sent_at IS NULL
ORDER BY created_at ASC, id ASC
LIMIT ?`

const fetchUnsentOutboxAfterSQL = `SELECT id, aggregate_id, destination, event_type, payload, created_at
FROM task_outbox
WHERE sent_at IS NULL AND (created_at > ? OR (created_at = ? AND id > ?))
ORDER BY created_at ASC, id ASC
LIMIT ?`

const markOutboxSentSQL = `UPDATE task_outbox SET sent_at = ? WHERE id = ? AND sent_at IS NULL`

// outboxDBTX is the subset of *sql.DB and *sql.Tx the outbox repository uses.
type outboxDBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type outboxRepo struct {
	db *sql.DB
}

// NewOutboxRepo returns the MySQL implementation of task.OutboxRepository. It
// uses plain database/sql (no sqlc) and needs parseTime=true in the DSN, which
// cmd/task already sets.
func NewOutboxRepo(db *sql.DB) task.OutboxRepository {
	return &outboxRepo{db: db}
}

// dbtx returns the transaction DoInTx stored in ctx, or the plain *sql.DB.
func (r *outboxRepo) dbtx(ctx context.Context) outboxDBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

func (r *outboxRepo) InsertUnsent(ctx context.Context, event task.OutboxEvent) error {
	_, err := r.dbtx(ctx).ExecContext(ctx, insertOutboxSQL,
		event.ID,
		event.AggregateID,
		event.Destination,
		event.EventType,
		event.Payload,
		// DATETIME(6) keeps microseconds; truncate so MySQL does not round up.
		event.CreatedAt.UTC().Truncate(time.Microsecond),
	)
	return err
}

func (r *outboxRepo) FetchUnsent(ctx context.Context, after task.OutboxCursor, limit int) ([]task.OutboxEvent, error) {
	// MySQL rejects a negative LIMIT.
	if limit <= 0 {
		return nil, nil
	}

	var (
		rows *sql.Rows
		err  error
	)
	if after.CreatedAt.IsZero() {
		rows, err = r.dbtx(ctx).QueryContext(ctx, fetchUnsentOutboxSQL, limit)
	} else {
		rows, err = r.dbtx(ctx).QueryContext(ctx, fetchUnsentOutboxAfterSQL,
			after.CreatedAt, after.CreatedAt, after.ID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []task.OutboxEvent
	for rows.Next() {
		var event task.OutboxEvent
		if err := rows.Scan(
			&event.ID,
			&event.AggregateID,
			&event.Destination,
			&event.EventType,
			&event.Payload,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *outboxRepo) MarkSent(ctx context.Context, id string) error {
	_, err := r.dbtx(ctx).ExecContext(ctx, markOutboxSentSQL, time.Now().UTC().Truncate(time.Microsecond), id)
	return err
}
