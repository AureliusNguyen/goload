package sqlite

import (
	"errors"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
)

// ownershipColumns are the task ownership columns in the order they are
// added. lease_expires_at and next_attempt_at hold fixed-width UTC text (see
// outboxTimeLayout). SQLite has no ADD COLUMN IF NOT EXISTS, so
// EnsureOwnershipSchema adds only the columns that are still missing.
var ownershipColumns = []struct {
	name string
	ddl  string
}{
	{"owner_id", `ALTER TABLE tasks ADD COLUMN owner_id TEXT;`},
	{"lease_expires_at", `ALTER TABLE tasks ADD COLUMN lease_expires_at TEXT;`},
	{"attempt_generation", `ALTER TABLE tasks ADD COLUMN attempt_generation INTEGER NOT NULL DEFAULT 0;`},
	{"retry_count", `ALTER TABLE tasks ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;`},
	{"next_attempt_at", `ALTER TABLE tasks ADD COLUMN next_attempt_at TEXT;`},
}

// EnsureOwnershipSchema adds the task ownership columns (owner, lease,
// generation, retry state) to an existing tasks table. It is idempotent; the
// pocket binaries call it on every start, after the tasks table is created.
// Existing rows read as unowned, generation 0, retry count 0.
func EnsureOwnershipSchema(conn *sqlite.Conn) error {
	existing := map[string]bool{}
	err := sqlitex.ExecuteTransient(conn, `PRAGMA table_info(tasks);`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			// table_info columns: cid, name, type, notnull, dflt_value, pk
			existing[stmt.ColumnText(1)] = true
			return nil
		},
	})
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		return errors.New("sqlite: tasks table does not exist; create it before the ownership columns")
	}

	for _, column := range ownershipColumns {
		if existing[column.name] {
			continue
		}
		if err := sqlitex.ExecuteTransient(conn, column.ddl, nil); err != nil {
			return err
		}
	}
	return nil
}
