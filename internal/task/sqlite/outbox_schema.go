package sqlite

import (
	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
)

// outboxTableDDL creates the transactional outbox. created_at and sent_at hold
// fixed-width UTC text (see outboxTimeLayout) so string order is time order.
const outboxTableDDL = `CREATE TABLE IF NOT EXISTS task_outbox (
    id TEXT PRIMARY KEY NOT NULL,
    aggregate_id INTEGER NOT NULL,
    destination TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload BLOB NOT NULL,
    created_at TEXT NOT NULL,
    sent_at TEXT
);`

const outboxUnsentIndexDDL = `CREATE INDEX IF NOT EXISTS task_outbox_unsent_idx ON task_outbox (sent_at, created_at);`

// EnsureOutboxSchema creates the task_outbox table and its index when they do
// not exist. It is idempotent; the pocket binaries call it on every start.
func EnsureOutboxSchema(conn *sqlite.Conn) error {
	if err := sqlitex.ExecuteTransient(conn, outboxTableDDL, nil); err != nil {
		return err
	}
	return sqlitex.ExecuteTransient(conn, outboxUnsentIndexDDL, nil)
}
