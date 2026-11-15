package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/stretchr/testify/require"

	task "github.com/yuisofull/goload/internal/task"
)

type taskColumn struct {
	Type    string
	NotNull bool
	Default string
}

// taskColumns reads PRAGMA table_info(tasks) as name -> definition.
func taskColumns(t *testing.T, conn *sqlite.Conn) map[string]taskColumn {
	t.Helper()

	columns := map[string]taskColumn{}
	require.NoError(t, sqlitex.ExecuteTransient(conn, `PRAGMA table_info(tasks);`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			// table_info columns: cid, name, type, notnull, dflt_value, pk
			columns[stmt.ColumnText(1)] = taskColumn{
				Type:    stmt.ColumnText(2),
				NotNull: stmt.ColumnInt(3) == 1,
				Default: stmt.ColumnText(4),
			}
			return nil
		},
	}))
	return columns
}

func TestEnsureOwnershipSchema_AddsColumnsAndIsIdempotent(t *testing.T) {
	pool := newTestPool(t)

	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	// newTestPool already ran it once; pocket runs it on every start.
	require.NoError(t, EnsureOwnershipSchema(conn))

	columns := taskColumns(t, conn)
	require.Equal(t, taskColumn{Type: "TEXT"}, columns["owner_id"])
	require.Equal(t, taskColumn{Type: "TEXT"}, columns["lease_expires_at"])
	require.Equal(t, taskColumn{Type: "INTEGER", NotNull: true, Default: "0"}, columns["attempt_generation"])
	require.Equal(t, taskColumn{Type: "INTEGER", NotNull: true, Default: "0"}, columns["retry_count"])
	require.Equal(t, taskColumn{Type: "TEXT"}, columns["next_attempt_at"])
	require.Equal(t, taskColumn{Type: "TEXT"}, columns["result_content_type"])
	require.Equal(t, taskColumn{Type: "INTEGER", NotNull: true, Default: "0"}, columns["result_file_size"])
	// The pre-existing columns are untouched.
	require.Equal(t, taskColumn{Type: "TEXT", NotNull: true}, columns["status"])
	require.Len(t, columns, 33, "26 original columns plus 5 ownership columns plus 2 result columns")
}

func TestEnsureOwnershipSchema_UpgradesExistingRows(t *testing.T) {
	pool := newBarePool(t)
	repo := NewTaskRepo(pool)
	ctx := context.Background()

	// A task written before the ownership columns existed.
	created, err := repo.Create(ctx, pendingTask(7))
	require.NoError(t, err)

	conn := pool.Get(ctx)
	require.NotNil(t, conn)
	defer pool.Put(conn)
	require.NoError(t, EnsureOwnershipSchema(conn))

	var (
		generation, retryCount         int64 = -1, -1
		ownerNull, leaseNull, nextNull bool
	)
	require.NoError(t, sqlitex.ExecuteTransient(
		conn,
		`SELECT owner_id, lease_expires_at, attempt_generation, retry_count, next_attempt_at FROM tasks WHERE id = ?;`,
		&sqlitex.ExecOptions{
			Args: []any{created.ID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				ownerNull = stmt.ColumnType(0) == sqlite.SQLITE_NULL
				leaseNull = stmt.ColumnType(1) == sqlite.SQLITE_NULL
				generation = stmt.ColumnInt64(2)
				retryCount = stmt.ColumnInt64(3)
				nextNull = stmt.ColumnType(4) == sqlite.SQLITE_NULL
				return nil
			},
		},
	))
	require.True(t, ownerNull, "an existing task starts unowned")
	require.True(t, leaseNull)
	require.Zero(t, generation)
	require.Zero(t, retryCount)
	require.True(t, nextNull)

	// The existing read path still works with the extra columns.
	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, task.StatusPending, got.Status)
	require.Equal(t, "file.bin", got.FileName)
}

func TestEnsureOwnershipSchema_RequiresTasksTable(t *testing.T) {
	pool, err := sqlitex.Open(filepath.Join(t.TempDir(), "goload_empty.db"), 0, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	err = EnsureOwnershipSchema(conn)

	require.ErrorContains(t, err, "tasks table does not exist")
}
