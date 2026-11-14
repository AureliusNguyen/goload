package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/stretchr/testify/require"
)

// tasksTableDDL mirrors the tasks table created by runMigrations in
// cmd/pocket/main.go and cmd/pocketsrv/main.go.
const tasksTableDDL = `CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    of_account_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    source_url TEXT NOT NULL,
    source_type TEXT NOT NULL,
    headers TEXT DEFAULT '{}',
    source_auth TEXT DEFAULT '{}',
    storage_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    checksum_type TEXT,
    checksum_value TEXT,
    concurrency INTEGER DEFAULT 4,
    max_speed INTEGER,
    max_retries INTEGER NOT NULL DEFAULT 3,
    timeout INTEGER,
    status TEXT NOT NULL,
    progress REAL DEFAULT 0.0,
    downloaded_bytes INTEGER DEFAULT 0,
    total_bytes INTEGER DEFAULT 0,
    error_message TEXT,
    metadata TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expiration_days INTEGER DEFAULT 30
);`

// newTestPool opens a throwaway on-disk SQLite pool that has the pocket
// tasks table (with the ownership columns) and the task_outbox table.
func newTestPool(t *testing.T) *sqlitex.Pool {
	t.Helper()

	pool, err := sqlitex.Open(filepath.Join(t.TempDir(), "goload_test.db"), 0, 4)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	require.NoError(t, sqlitex.ExecuteTransient(conn, tasksTableDDL, nil))
	require.NoError(t, EnsureOutboxSchema(conn))
	require.NoError(t, EnsureOwnershipSchema(conn))

	return pool
}

// newBarePool opens a pool that has only the tasks table as it existed
// before the ownership columns: the upgrade path of an existing database.
func newBarePool(t *testing.T) *sqlitex.Pool {
	t.Helper()

	pool, err := sqlitex.Open(filepath.Join(t.TempDir(), "goload_bare.db"), 0, 2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	require.NoError(t, sqlitex.ExecuteTransient(conn, tasksTableDDL, nil))

	return pool
}
