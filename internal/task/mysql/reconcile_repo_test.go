package mysql

import (
	"context"
	"database/sql/driver"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	task "github.com/yuisofull/goload/internal/task"
)

func TestMigration0005_TaskReconcileIndexIsIdempotent(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "mysql", "0005.task_reconcile_index.sql"))
	require.NoError(t, err)
	ddl := strings.Join(strings.Fields(string(raw)), " ")

	// deployments/run-migrations.sh re-applies every file on each start, and
	// MySQL has no CREATE INDEX IF NOT EXISTS: the index is added through a
	// prepared statement guarded by an information_schema lookup.
	require.Contains(t, ddl, "FROM information_schema.statistics")
	require.Contains(t, ddl, "AND table_name = 'tasks'")
	require.Contains(t, ddl, "AND index_name = 'tasks_reconcile_idx'")
	require.Contains(t, ddl, "'ALTER TABLE tasks ADD INDEX tasks_reconcile_idx (status, lease_expires_at)'")
	require.Equal(t, 1, strings.Count(ddl, "PREPARE stmt FROM @sqlstmt;"))
	require.Equal(t, 1, strings.Count(ddl, "EXECUTE stmt;"))
	require.Equal(t, 1, strings.Count(ddl, "DEALLOCATE PREPARE stmt;"))
	require.Equal(t, strings.Count(ddl, "ALTER TABLE"), strings.Count(ddl, "'ALTER TABLE"),
		"every ALTER TABLE must run through a guarded prepared statement")
	require.NotContains(t, strings.ToUpper(ddl), "DROP ")
	require.NotContains(t, string(raw), strings.Repeat("-", 2), "comments in this file start with #")
}

func TestReconcileRepo_ListReconcilable_QueriesClaimableTasks(t *testing.T) {
	fake, db := openFakeDB(t)
	fake.columns = []string{"id", "attempt_generation"}
	fake.rows = [][]driver.Value{
		{int64(7), int64(0)},
		{int64(9), int64(3)},
	}
	// Nanoseconds prove the bound is truncated to DATETIME(6) precision.
	now := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)

	got, err := NewReconcileRepo(db).ListReconcilable(context.Background(), now, 5, 50)

	require.NoError(t, err)
	require.Equal(t, []task.ReconcileCandidate{{TaskID: 7, Generation: 0}, {TaskID: 9, Generation: 3}}, got)

	steps, calls := fake.snapshot()
	require.Equal(t, []string{"query"}, steps)
	require.Len(t, calls, 1)
	require.False(t, calls[0].inTx, "listing candidates needs no transaction")
	query := strings.Join(strings.Fields(calls[0].query), " ")
	require.Contains(t, query, "SELECT id, attempt_generation FROM tasks WHERE id > ?")
	require.Contains(t, query,
		"(status = 'PENDING' AND owner_id IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= ?))")
	require.Contains(t, query,
		"OR (status IN ('DOWNLOADING', 'STORING') AND (lease_expires_at IS NULL OR lease_expires_at <= ?))")
	require.True(t, strings.HasSuffix(query, "ORDER BY id ASC LIMIT ?"))
	for _, excluded := range []string{"PAUSED", "CANCELLED", "COMPLETED", "FAILED"} {
		require.NotContains(t, query, excluded)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC)
	require.Equal(t, []driver.Value{int64(5), at, at, int64(50)}, calls[0].args)
}

func TestReconcileRepo_ListReconcilable_NonPositiveLimitSkipsQuery(t *testing.T) {
	fake, db := openFakeDB(t)
	repo := NewReconcileRepo(db)

	// MySQL rejects a negative LIMIT; the repository answers without asking.
	for _, limit := range []int{0, -1} {
		got, err := repo.ListReconcilable(context.Background(), time.Now(), 0, limit)
		require.NoError(t, err)
		require.Empty(t, got)
	}

	steps, _ := fake.snapshot()
	require.Empty(t, steps)
}
