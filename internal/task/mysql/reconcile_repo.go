package mysql

import (
	"context"
	"database/sql"
	"math"
	"time"

	task "github.com/yuisofull/goload/internal/task"
	"github.com/yuisofull/goload/internal/task/mysql/sqlc"
)

// reconcileRepo implements task.ReconcileRepository. The listing is
// read-only, so it needs neither a transaction nor a row lock.
type reconcileRepo struct {
	queries *sqlc.Queries
}

// NewReconcileRepo returns the MySQL implementation of task.ReconcileRepository.
// It uses the sqlc-generated queries and needs parseTime=true in the DSN.
func NewReconcileRepo(db *sql.DB) task.ReconcileRepository {
	return &reconcileRepo{queries: sqlc.New(db)}
}

func (r *reconcileRepo) ListReconcilable(
	ctx context.Context,
	now time.Time,
	afterID uint64,
	limit int,
) ([]task.ReconcileCandidate, error) {
	// MySQL rejects a negative LIMIT.
	if limit <= 0 {
		return nil, nil
	}
	if limit > math.MaxInt32 {
		limit = math.MaxInt32
	}

	// The lease and retry columns are DATETIME(6): compare at that precision.
	at := sql.NullTime{Time: now.UTC().Truncate(time.Microsecond), Valid: true}
	rows, err := r.queries.ListReconcilable(ctx, sqlc.ListReconcilableParams{
		ID:             afterID,
		NextAttemptAt:  at,
		LeaseExpiresAt: at,
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, err
	}

	var candidates []task.ReconcileCandidate
	for _, row := range rows {
		candidates = append(candidates, task.ReconcileCandidate{TaskID: row.ID, Generation: row.AttemptGeneration})
	}
	return candidates, nil
}
