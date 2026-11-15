package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/stretchr/testify/require"

	task "github.com/yuisofull/goload/internal/task"
)

// reconcilable lists every reconcilable task at the fixture clock's now.
func (f *attemptFixture) reconcilable(t *testing.T) []task.ReconcileCandidate {
	t.Helper()

	got, err := f.reconcile.ListReconcilable(context.Background(), f.clock.Now(), 0, 100)
	require.NoError(t, err)
	return got
}

// T1 at the query level: a task whose dispatch was lost before any worker
// claimed it is PENDING, unowned and due.
func TestListReconcilable_PendingUnownedDueTask(t *testing.T) {
	f := newAttemptFixture(t)
	id := f.newTask(t)

	require.Equal(t, []task.ReconcileCandidate{{TaskID: id, Generation: 0}}, f.reconcilable(t))
}

// T2 at the query level: the owner stops renewing and the lease runs out.
func TestListReconcilable_LeaseBoundary(t *testing.T) {
	f := newAttemptFixture(t)
	id := f.newTask(t)
	_, err := f.svc.ClaimTask(context.Background(), id, "worker-a")
	require.NoError(t, err)
	require.Empty(t, f.reconcilable(t), "a task with a live lease has an owner")

	f.clock.Advance(task.DefaultLeaseDuration - time.Nanosecond)
	require.Empty(t, f.reconcilable(t), "one nanosecond before expiry the owner still holds the task")

	// A lease is expired from the instant now reaches lease_expires_at.
	f.clock.Advance(time.Nanosecond)
	require.Equal(t, []task.ReconcileCandidate{{TaskID: id, Generation: 1}}, f.reconcilable(t))
}

func TestListReconcilable_StoringWithExpiredLease(t *testing.T) {
	f := newAttemptFixture(t)
	id := f.newTask(t)
	f.claimStoring(t, id, "worker-a")

	f.clock.Advance(task.DefaultLeaseDuration)

	require.Equal(t, []task.ReconcileCandidate{{TaskID: id, Generation: 1}}, f.reconcilable(t))
}

func TestListReconcilable_RetryOnlyWhenDue(t *testing.T) {
	f := newAttemptFixture(t)
	id := f.newTask(t)
	ctx := context.Background()
	attempt, err := f.svc.ClaimTask(ctx, id, "worker-a")
	require.NoError(t, err)
	require.NoError(t, f.svc.FailAttempt(ctx, id, "worker-a", attempt.Generation, errors.New("connection reset")))
	require.Empty(t, f.reconcilable(t), "a retry that is not due yet must wait for its backoff")

	f.clock.Advance(task.DefaultRetryBackoff(1) - time.Nanosecond)
	require.Empty(t, f.reconcilable(t))

	f.clock.Advance(time.Nanosecond)
	require.Equal(t, []task.ReconcileCandidate{{TaskID: id, Generation: 1}}, f.reconcilable(t))
}

func TestListReconcilable_ExcludesHeldAndTerminalTasks(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()

	paused := f.newTask(t)
	_, err := f.svc.ClaimTask(ctx, paused, "worker-a")
	require.NoError(t, err)
	require.NoError(t, f.tasks.PauseTask(ctx, paused))

	cancelled := f.newTask(t)
	require.NoError(t, f.tasks.CancelTask(ctx, cancelled))

	completed := f.newTask(t)
	attempt := f.claimStoring(t, completed, "worker-a")
	require.NoError(t, f.svc.CompleteAttempt(
		ctx, completed, "worker-a", attempt.Generation, attemptResult(completed, attempt.Generation),
	))

	noRetries := pendingTask(7)
	noRetries.DownloadOptions.MaxRetries = 0
	created, err := f.repo.Create(ctx, noRetries)
	require.NoError(t, err)
	failed := created.ID
	attempt, err = f.svc.ClaimTask(ctx, failed, "worker-a")
	require.NoError(t, err)
	require.NoError(t, f.svc.FailAttempt(ctx, failed, "worker-a", attempt.Generation, errors.New("boom")))

	for id, want := range map[uint64]task.TaskStatus{
		paused:    task.StatusPaused,
		cancelled: task.StatusCancelled,
		completed: task.StatusCompleted,
		failed:    task.StatusFailed,
	} {
		require.Equal(t, want, f.state(t, id).Status)
	}

	// However much time passes, none of them is ever dispatched again.
	require.Empty(t, f.reconcilable(t))
	f.clock.Advance(24 * time.Hour)
	require.Empty(t, f.reconcilable(t))
}

func TestListReconcilable_DownloadingWithoutLeaseIsReconcilable(t *testing.T) {
	f := newAttemptFixture(t)
	id := f.newTask(t)
	// A row put into DOWNLOADING by the legacy unfenced status update has no
	// owner and no lease. ClaimTask accepts it, so it must be listed.
	_, err := f.repo.Update(context.Background(), &task.Task{ID: id, Status: task.StatusDownloading})
	require.NoError(t, err)

	require.Equal(t, []task.ReconcileCandidate{{TaskID: id, Generation: 0}}, f.reconcilable(t))
}

func TestListReconcilable_PagesInIDOrder(t *testing.T) {
	f := newAttemptFixture(t)
	ctx := context.Background()
	ids := []uint64{f.newTask(t), f.newTask(t), f.newTask(t)}
	now := f.clock.Now()

	first, err := f.reconcile.ListReconcilable(ctx, now, 0, 2)
	require.NoError(t, err)
	require.Equal(t, []task.ReconcileCandidate{{TaskID: ids[0]}, {TaskID: ids[1]}}, first)

	rest, err := f.reconcile.ListReconcilable(ctx, now, ids[1], 2)
	require.NoError(t, err)
	require.Equal(t, []task.ReconcileCandidate{{TaskID: ids[2]}}, rest)

	// SQLite reads a negative LIMIT as "no limit": the repository must not.
	for _, limit := range []int{0, -1} {
		none, err := f.reconcile.ListReconcilable(ctx, now, 0, limit)
		require.NoError(t, err)
		require.Empty(t, none)
	}
}

func TestEnsureOwnershipSchema_CreatesReconcileIndex(t *testing.T) {
	// The upgrade path: a database that has the tasks table but no index yet.
	pool := newBarePool(t)
	conn := pool.Get(context.Background())
	require.NotNil(t, conn)
	defer pool.Put(conn)

	require.NoError(t, EnsureOwnershipSchema(conn))
	// Pocket runs it on every start, so a second run must be a no-op.
	require.NoError(t, EnsureOwnershipSchema(conn))

	var columns []string
	require.NoError(t, sqlitex.ExecuteTransient(conn, `PRAGMA index_info(tasks_reconcile_idx);`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			// index_info columns: seqno, cid, name
			columns = append(columns, stmt.ColumnText(2))
			return nil
		},
	}))
	require.Equal(t, []string{"status", "lease_expires_at"}, columns)
}
