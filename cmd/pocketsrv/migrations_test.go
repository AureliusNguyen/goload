package main

import (
	"context"
	"path/filepath"
	"testing"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/stretchr/testify/require"
)

func TestRunMigrations_CreatesTaskOutbox(t *testing.T) {
	pool, err := sqlitex.Open(filepath.Join(t.TempDir(), "pocketsrv_test.db"), 0, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	require.NoError(t, runMigrations(pool))
	// Pocket runs migrations on every start, so a second run must be a no-op.
	require.NoError(t, runMigrations(pool))

	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	var tables []string
	require.NoError(t, sqlitex.ExecuteTransient(
		conn,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name IN ('tasks', 'task_outbox') ORDER BY name;`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				tables = append(tables, stmt.ColumnText(0))
				return nil
			},
		},
	))
	require.Equal(t, []string{"task_outbox", "tasks"}, tables)
}

func TestRunMigrations_AddsOwnershipColumns(t *testing.T) {
	pool, err := sqlitex.Open(filepath.Join(t.TempDir(), "pocketsrv_ownership_test.db"), 0, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	require.NoError(t, runMigrations(pool))
	// Pocket runs migrations on every start, so a second run must be a no-op.
	require.NoError(t, runMigrations(pool))

	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	columns := map[string]bool{}
	require.NoError(t, sqlitex.ExecuteTransient(conn, `PRAGMA table_info(tasks);`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			columns[stmt.ColumnText(1)] = true
			return nil
		},
	}))
	for _, name := range []string{
		"owner_id", "lease_expires_at", "attempt_generation", "retry_count", "next_attempt_at",
	} {
		require.True(t, columns[name], "tasks.%s must exist after runMigrations", name)
	}
}
