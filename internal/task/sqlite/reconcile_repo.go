package sqlite

import (
	"context"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"

	task "github.com/yuisofull/goload/internal/task"
)

// listReconcilableSQL selects the claimable tasks. ?1 is the id to start
// after, ?2 is now as fixed-width UTC text (the format the two time columns
// hold, so comparing the strings compares the times), ?3 is the page size.
const listReconcilableSQL = `SELECT id, attempt_generation
FROM tasks
WHERE id > ?1
  AND (
        (status = 'PENDING' AND owner_id IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= ?2))
     OR (status IN ('DOWNLOADING', 'STORING') AND (lease_expires_at IS NULL OR lease_expires_at <= ?2))
      )
ORDER BY id ASC
LIMIT ?3`

// NewReconcileRepo returns the SQLite implementation of task.ReconcileRepository.
func NewReconcileRepo(pool *sqlitex.Pool) task.ReconcileRepository {
	return &attemptRepo{pool: pool}
}

func (r *attemptRepo) ListReconcilable(
	ctx context.Context,
	now time.Time,
	afterID uint64,
	limit int,
) ([]task.ReconcileCandidate, error) {
	// SQLite reads a negative LIMIT as "no limit".
	if limit <= 0 {
		return nil, nil
	}

	conn := r.pool.Get(ctx)
	if conn == nil {
		return nil, context.DeadlineExceeded
	}
	defer r.pool.Put(conn)

	var candidates []task.ReconcileCandidate
	err := sqlitex.Execute(conn, listReconcilableSQL, &sqlitex.ExecOptions{
		Args: []any{afterID, now.UTC().Format(outboxTimeLayout), limit},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			candidates = append(candidates, task.ReconcileCandidate{
				TaskID:     uint64(stmt.ColumnInt64(0)),
				Generation: uint64(stmt.ColumnInt64(1)),
			})
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return candidates, nil
}
