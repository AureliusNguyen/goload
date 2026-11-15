package mysql

import (
	"context"
	"database/sql"
	"time"

	task "github.com/yuisofull/goload/internal/task"
)

// listReconcilableSQL selects the claimable tasks. The arguments are: the id
// to start after, now (twice) and the page size.
const listReconcilableSQL = `SELECT id, attempt_generation
FROM tasks
WHERE id > ?
  AND (
        (status = 'PENDING' AND owner_id IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= ?))
     OR (status IN ('DOWNLOADING', 'STORING') AND (lease_expires_at IS NULL OR lease_expires_at <= ?))
      )
ORDER BY id ASC
LIMIT ?`

// NewReconcileRepo returns the MySQL implementation of task.ReconcileRepository.
// Like the attempt repository it uses plain database/sql and needs
// parseTime=true in the DSN.
func NewReconcileRepo(db *sql.DB) task.ReconcileRepository {
	return &attemptRepo{db: db}
}

func (r *attemptRepo) ListReconcilable(
	ctx context.Context,
	now time.Time,
	afterID uint64,
	limit int,
) ([]task.ReconcileCandidate, error) {
	// MySQL rejects a negative LIMIT.
	if limit <= 0 {
		return nil, nil
	}

	// The lease and retry columns are DATETIME(6): compare at that precision.
	at := now.UTC().Truncate(time.Microsecond)
	rows, err := r.db.QueryContext(ctx, listReconcilableSQL, afterID, at, at, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []task.ReconcileCandidate
	for rows.Next() {
		var candidate task.ReconcileCandidate
		if err := rows.Scan(&candidate.TaskID, &candidate.Generation); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}
