package sqlite

import (
	"context"
	"fmt"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"

	task "github.com/yuisofull/goload/internal/task"
)

// outboxTimeLayout stores timestamps as fixed-width UTC text, so comparing
// the strings compares the times.
const outboxTimeLayout = "2006-01-02T15:04:05.000000000Z"

type outboxRepo struct {
	pool *sqlitex.Pool
}

// NewOutboxRepo returns the SQLite implementation of task.OutboxRepository.
func NewOutboxRepo(pool *sqlitex.Pool) task.OutboxRepository {
	return &outboxRepo{pool: pool}
}

// withConn runs fn on the connection DoInTx stored in ctx, or on a pooled
// connection when ctx carries none.
func (r *outboxRepo) withConn(ctx context.Context, fn func(conn *sqlite.Conn) error) error {
	if conn, ok := ctx.Value(connKey{}).(*sqlite.Conn); ok {
		return fn(conn)
	}
	conn := r.pool.Get(ctx)
	if conn == nil {
		return context.DeadlineExceeded
	}
	defer r.pool.Put(conn)
	return fn(conn)
}

func (r *outboxRepo) InsertUnsent(ctx context.Context, event task.OutboxEvent) error {
	return r.withConn(ctx, func(conn *sqlite.Conn) error {
		return sqlitex.Execute(
			conn,
			`INSERT INTO task_outbox (id, aggregate_id, destination, event_type, payload, created_at, sent_at)
VALUES (?, ?, ?, ?, ?, ?, NULL)`,
			&sqlitex.ExecOptions{
				Args: []any{
					event.ID, event.AggregateID, event.Destination, event.EventType, event.Payload,
					event.CreatedAt.UTC().Format(outboxTimeLayout),
				},
			},
		)
	})
}

func (r *outboxRepo) FetchUnsent(ctx context.Context, limit int) ([]task.OutboxEvent, error) {
	// SQLite reads a negative LIMIT as "no limit".
	if limit <= 0 {
		return nil, nil
	}

	var events []task.OutboxEvent
	err := r.withConn(ctx, func(conn *sqlite.Conn) error {
		return sqlitex.Execute(
			conn,
			`SELECT id, aggregate_id, destination, event_type, payload, created_at
FROM task_outbox
WHERE sent_at IS NULL
ORDER BY created_at ASC, rowid ASC
LIMIT ?`,
			&sqlitex.ExecOptions{
				Args: []any{limit},
				ResultFunc: func(stmt *sqlite.Stmt) error {
					event, err := scanOutboxEvent(stmt)
					if err != nil {
						return err
					}
					events = append(events, event)
					return nil
				},
			},
		)
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *outboxRepo) MarkSent(ctx context.Context, id string) error {
	return r.withConn(ctx, func(conn *sqlite.Conn) error {
		return sqlitex.Execute(
			conn,
			`UPDATE task_outbox SET sent_at = ? WHERE id = ? AND sent_at IS NULL`,
			&sqlitex.ExecOptions{Args: []any{time.Now().UTC().Format(outboxTimeLayout), id}},
		)
	})
}

// scanOutboxEvent maps one FetchUnsent row. Column order: id, aggregate_id,
// destination, event_type, payload, created_at.
func scanOutboxEvent(stmt *sqlite.Stmt) (task.OutboxEvent, error) {
	payload := make([]byte, stmt.ColumnLen(4))
	stmt.ColumnBytes(4, payload)

	createdAt, err := time.Parse(outboxTimeLayout, stmt.ColumnText(5))
	if err != nil {
		return task.OutboxEvent{}, fmt.Errorf("parse outbox created_at: %w", err)
	}

	return task.OutboxEvent{
		ID:          stmt.ColumnText(0),
		AggregateID: uint64(stmt.ColumnInt64(1)),
		Destination: stmt.ColumnText(2),
		EventType:   stmt.ColumnText(3),
		Payload:     payload,
		CreatedAt:   createdAt,
	}, nil
}
