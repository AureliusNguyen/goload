package main

// Worker-level crash recovery for pocket mode.
//
// Simulating a crash correctly is the point of this file. A graceful
// shutdown is NOT a crash: when a worker's context ends, runAttempt calls
// ReleaseAttempt, which hands the task back at once. A real crash (kill -9,
// power loss) runs no code at all, so the lease stays HELD by the dead worker
// and recovery has to wait for the lease to EXPIRE before the reconciler
// dispatches the task again.
//
// The tests therefore put an attemptGate between the worker and the real
// ownership API. Killing the gate makes every later call of that worker fail
// without reaching the database, exactly as if the process were gone. The
// worker's goroutines are then torn down (the test process has to reap them),
// but whatever they try to write, ReleaseAttempt included, is dropped. Each
// crash test asserts, before any recovery, that the row still shows the dead
// worker as owner with an unexpired lease, and that the dropped
// ReleaseAttempt never reached the database.
//
// Lease expiry is driven by a fake clock shared by every pipeline of a test;
// nothing waits on wall-clock lease time.

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/internal/download"
	"github.com/yuisofull/goload/internal/events"
	"github.com/yuisofull/goload/internal/task"
	"github.com/yuisofull/goload/pkg/message"
)

const crashTestLease = 10 * time.Second

// testClock is the clock of the ownership API and the reconciler. Advance
// expires leases without sleeping.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var errWorkerGone = errors.New("worker process is gone")

// gateCall is one ownership call a worker made through the gate.
type gateCall struct {
	Method string
	// Dropped calls never reached the ownership API (the worker was down).
	Dropped bool
	Err     error
}

// attemptGate wraps the ownership API a worker uses. While down, every call
// is dropped with errWorkerGone and nothing reaches the database: the worker
// is dead (a crash) or cut off (a partition). It records every call.
type attemptGate struct {
	inner download.AttemptCoordinator
	down  atomic.Bool

	// loseNextComplete makes the next CompleteAttempt commit in the inner
	// service but report an error to the caller (a lost response); then
	// afterLost, if set, runs.
	loseNextComplete atomic.Bool
	afterLost        func()

	mu    sync.Mutex
	calls []gateCall
}

func (g *attemptGate) record(method string, dropped bool, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, gateCall{Method: method, Dropped: dropped, Err: err})
}

// count returns how many calls of method were dropped (dropped true) or
// forwarded to the ownership API (false).
func (g *attemptGate) count(method string, dropped bool) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.calls {
		if c.Method == method && c.Dropped == dropped {
			n++
		}
	}
	return n
}

// rejected counts forwarded calls of method the ownership API rejected as
// fenced.
func (g *attemptGate) rejected(method string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.calls {
		if c.Method == method && !c.Dropped && errors.Is(c.Err, events.ErrAttemptRejected) {
			n++
		}
	}
	return n
}

func (g *attemptGate) forwardedCalls(method string) []gateCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []gateCall
	for _, c := range g.calls {
		if c.Method == method && !c.Dropped {
			out = append(out, c)
		}
	}
	return out
}

func (g *attemptGate) ClaimTask(ctx context.Context, taskID uint64, workerID string) (*events.Attempt, error) {
	if g.down.Load() {
		g.record("ClaimTask", true, errWorkerGone)
		return nil, errWorkerGone
	}
	a, err := g.inner.ClaimTask(ctx, taskID, workerID)
	g.record("ClaimTask", false, err)
	return a, err
}

func (g *attemptGate) RenewLease(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
	update events.LeaseUpdate,
) (time.Time, error) {
	if g.down.Load() {
		g.record("RenewLease", true, errWorkerGone)
		return time.Time{}, errWorkerGone
	}
	lease, err := g.inner.RenewLease(ctx, taskID, workerID, generation, update)
	g.record("RenewLease", false, err)
	return lease, err
}

func (g *attemptGate) CompleteAttempt(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
	result events.AttemptResult,
) error {
	if g.down.Load() {
		g.record("CompleteAttempt", true, errWorkerGone)
		return errWorkerGone
	}
	err := g.inner.CompleteAttempt(ctx, taskID, workerID, generation, result)
	g.record("CompleteAttempt", false, err)
	if err == nil && g.loseNextComplete.CompareAndSwap(true, false) {
		// The commit happened; only its response is lost.
		if g.afterLost != nil {
			g.afterLost()
		}
		return errors.New("connection reset before the response arrived")
	}
	return err
}

func (g *attemptGate) FailAttempt(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
	cause error,
) error {
	if g.down.Load() {
		g.record("FailAttempt", true, errWorkerGone)
		return errWorkerGone
	}
	err := g.inner.FailAttempt(ctx, taskID, workerID, generation, cause)
	g.record("FailAttempt", false, err)
	return err
}

func (g *attemptGate) ReleaseAttempt(ctx context.Context, taskID uint64, workerID string, generation uint64) error {
	if g.down.Load() {
		g.record("ReleaseAttempt", true, errWorkerGone)
		return errWorkerGone
	}
	err := g.inner.ReleaseAttempt(ctx, taskID, workerID, generation)
	g.record("ReleaseAttempt", false, err)
	return err
}

// crash simulates the death of the whole process of p: from now on nothing
// its worker does reaches the database, then every goroutine is reaped. The
// pool and the store stay open so the test can inspect the row afterwards;
// the cleanup closes them. See the file comment for why this is a crash and
// not a shutdown.
func (p *pocketPipeline) crash(t *testing.T) {
	t.Helper()
	require.NotNil(t, p.gate, "only a gated pipeline can be crashed")
	p.gate.down.Store(true)
	p.stopAll()
}

// outboxRows counts every row (sent or not) of eventType for the task.
func (p *pocketPipeline) outboxRows(t *testing.T, taskID uint64, eventType string) int {
	t.Helper()

	conn := p.pool.Get(context.Background())
	require.NotNil(t, conn)
	defer p.pool.Put(conn)

	var n int
	err := sqlitex.Exec(conn,
		`SELECT COUNT(*) FROM task_outbox WHERE aggregate_id = ? AND event_type = ?`,
		func(stmt *sqlite.Stmt) error {
			n = int(stmt.ColumnInt64(0))
			return nil
		}, int64(taskID), eventType)
	require.NoError(t, err)
	return n
}

// objects lists the stored objects of the task (any generation), ignoring the
// .meta.json sidecar the local backend writes next to each object.
func (p *pocketPipeline) objects(t *testing.T, taskID uint64) []string {
	t.Helper()

	var found []string
	root := filepath.Join(p.dataDir, strconv.FormatUint(taskID, 10))
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() && !strings.HasSuffix(path, ".tmp") && !strings.HasSuffix(path, ".meta.json") {
			rel, _ := filepath.Rel(p.dataDir, path)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

func (p *pocketPipeline) waitOutboxDrained(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		unsent, err := p.outbox.FetchUnsent(context.Background(), task.OutboxCursor{}, 10)
		return err == nil && len(unsent) == 0
	}, 5*time.Second, 10*time.Millisecond, "the outbox never drained")
}

// crashEnv is the shared substrate of one test: a clock, a db file and an
// object store directory that survive the death of a pipeline.
type crashEnv struct {
	clock           *testClock
	dbPath, dataDir string
}

func newCrashEnv(t *testing.T) *crashEnv {
	t.Helper()
	return &crashEnv{
		clock:   newTestClock(),
		dbPath:  filepath.Join(t.TempDir(), "shared.db"),
		dataDir: filepath.Join(t.TempDir(), "data"),
	}
}

func (e *crashEnv) opts(workerID string) pipelineOpts {
	return pipelineOpts{
		dbPath:   e.dbPath,
		dataDir:  e.dataDir,
		workerID: workerID,
		clock:    e.clock.Now,
		lease:    crashTestLease,
		gated:    true,
	}
}

// T1: the dispatch reaches the worker but the worker dies before its claim
// commits. Nothing is owned, so there is no lease to wait for: the
// reconciler dispatches the PENDING task again and a second worker finishes
// it. Exactly one result.
func TestCrashRecovery_T1_CrashBeforeClaim(t *testing.T) {
	env := newCrashEnv(t)
	dlA := &scriptedDownloader{body: "t1 payload"}
	aOpts := env.opts("worker-a")
	aOpts.noReconciler = true // A is the only thing that would dispatch; keep the recovery to B
	a := startPipeline(t, dlA, aOpts)

	// Worker A is dead before it can commit any claim.
	a.gate.down.Store(true)
	id := a.create(t)

	require.Eventually(t, func() bool { return a.gate.count("ClaimTask", true) >= 1 },
		5*time.Second, 5*time.Millisecond, "the dispatch never reached worker A")
	a.waitOutboxDrained(t) // the dispatch was published: only the reconciler can recover now
	a.crash(t)

	st := a.state(t, id)
	require.Equal(t, task.StatusPending, st.Status)
	require.Zero(t, st.Generation, "the claim never committed")
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Zero(t, dlA.starts.Load())

	dlB := &scriptedDownloader{body: "t1 payload"}
	bOpts := env.opts("worker-b")
	bOpts.gated = false
	b := startPipeline(t, dlB, bOpts)

	done := b.waitFor(t, id, task.StatusCompleted)
	b.waitOutboxDrained(t)

	require.Equal(t, uint64(1), done.Generation, "the dead worker's claim never committed, so B holds generation 1")
	require.True(t, strings.HasPrefix(done.StoragePath, strconv.FormatUint(id, 10)+"/1/file-"), done.StoragePath)
	require.Equal(t, done, b.state(t, id), "the completed row never changes again")
	require.Zero(t, dlA.starts.Load())
	require.Equal(t, int32(1), dlB.starts.Load(), "exactly one download")
	require.Equal(t, 1, b.outboxRows(t, id, task.OutboxEventTaskCompleted), "exactly one completion event")
	require.Len(t, b.objects(t, id), 1, "exactly one stored object")
	require.Equal(t, 2, b.outboxRows(t, id, task.OutboxEventTaskCreated),
		"the original dispatch plus one reconciler redispatch")
}

// T2: worker A claims generation 1, is mid-download, then crashes with its
// lease held. The row keeps showing A as owner until the lease expires; only
// then does the reconciler dispatch the task, and worker B takes generation 2
// and downloads from zero. Exactly one result, committed under generation 2.
func TestCrashRecovery_T2_CrashDuringDownload(t *testing.T) {
	env := newCrashEnv(t)
	dlA := newBlockableDownloader("t2 payload")
	a := startPipeline(t, dlA, env.opts("worker-a"))

	id := a.create(t)
	<-dlA.entered // A claimed generation 1 and is blocked inside Download
	claimed := a.state(t, id)
	require.Equal(t, task.StatusDownloading, claimed.Status)
	require.Equal(t, uint64(1), claimed.Generation)

	// The crash. See the file comment: this is NOT a graceful stop.
	a.crash(t)

	// PROOF THAT THIS WAS A CRASH AND NOT A SHUTDOWN. The dying worker did
	// reach for ReleaseAttempt (a graceful stop would have run it), the gate
	// dropped it, and the row still shows A as the owner at generation 1 with
	// an unexpired lease: nothing was released.
	require.Equal(t, 1, a.gate.count("ReleaseAttempt", true), "the dying worker tried to release and was dropped")
	require.Zero(t, a.gate.count("ReleaseAttempt", false), "no ReleaseAttempt may reach the database")
	require.Zero(t, a.gate.count("FailAttempt", false))
	held := a.state(t, id)
	require.Equal(t, task.StatusDownloading, held.Status, "task must not have been handed back")
	require.Equal(t, "worker-a", held.OwnerID, "the dead worker must still own the task")
	require.Equal(t, uint64(1), held.Generation)
	require.NotNil(t, held.LeaseExpiresAt)
	require.True(t, held.LeaseExpiresAt.After(env.clock.Now()),
		"the lease must still be unexpired right after the crash, got expiry %s at now %s",
		held.LeaseExpiresAt, env.clock.Now())

	// Restart: pipeline B on the same db file. Its reconciler is running.
	dlB := &scriptedDownloader{body: "t2 payload"}
	bOpts := env.opts("worker-b")
	bOpts.gated = false
	b := startPipeline(t, dlB, bOpts)

	// While the lease is live the reconciler has nothing to recover.
	cands, err := b.reconcile.ListReconcilable(context.Background(), env.clock.Now(), 0, 10)
	require.NoError(t, err)
	require.Empty(t, cands, "a held, unexpired lease is not reconcilable")
	require.Never(t, func() bool {
		st := b.state(t, id)
		return st.Generation != 1 || st.OwnerID != "worker-a" || st.Status != task.StatusDownloading
	}, 300*time.Millisecond, 20*time.Millisecond, "nobody may take the task before its lease expires")
	require.Zero(t, dlB.starts.Load())

	// Just before expiry: still held.
	env.clock.Advance(crashTestLease - time.Second)
	cands, err = b.reconcile.ListReconcilable(context.Background(), env.clock.Now(), 0, 10)
	require.NoError(t, err)
	require.Empty(t, cands)

	// Past expiry the reconciler recovers it.
	env.clock.Advance(2 * time.Second)
	cands, err = b.reconcile.ListReconcilable(context.Background(), env.clock.Now(), 0, 10)
	require.NoError(t, err)
	require.Equal(t, []task.ReconcileCandidate{{TaskID: id, Generation: 1}}, cands)

	done := b.waitFor(t, id, task.StatusCompleted)
	b.waitOutboxDrained(t)

	require.Equal(t, uint64(2), done.Generation, "B claimed generation G+1")
	require.True(t, strings.HasPrefix(done.StoragePath, strconv.FormatUint(id, 10)+"/2/file-"),
		"the result is committed under generation 2, got %q", done.StoragePath)
	require.Empty(t, done.OwnerID)
	require.Equal(t, int32(1), dlB.starts.Load(), "B downloads once, from zero")
	require.Equal(t, 1, b.outboxRows(t, id, task.OutboxEventTaskCompleted), "exactly one completion event")
	require.Len(t, b.objects(t, id), 1, "exactly one object, A's blocked download stored nothing")
	require.Equal(t, done, b.state(t, id))
}

// stageTakeover builds the fencing scenario shared by T3 and T3b. Worker A
// claims generation 1 and is held mid-download; it is then cut off (alive but
// unable to renew) and its control plane stops so only B dispatches. The
// clock passes A's lease, B recovers the task as generation 2 and completes.
// A is still alive, blocked in its download, when this returns.
func stageTakeover(t *testing.T) (env *crashEnv, a, b *pocketPipeline, dlA *scriptedDownloader, id uint64, committed *task.AttemptState) {
	t.Helper()

	env = newCrashEnv(t)
	dlA = newBlockableDownloader("takeover payload")
	aOpts := env.opts("worker-a")
	aOpts.noReconciler = true
	a = startPipeline(t, dlA, aOpts)

	id = a.create(t)
	<-dlA.entered
	require.Equal(t, uint64(1), a.state(t, id).Generation)

	// A is cut off but alive; its relay and lifecycle consumer stop so they
	// cannot steal B's dispatch from the shared outbox.
	a.gate.down.Store(true)
	a.stopControl()
	a.control.Wait()
	env.clock.Advance(crashTestLease + time.Second)

	bOpts := env.opts("worker-b")
	bOpts.gated = false
	b = startPipeline(t, &scriptedDownloader{body: "takeover payload"}, bOpts)
	committed = b.waitFor(t, id, task.StatusCompleted)
	b.waitOutboxDrained(t)
	require.Equal(t, uint64(2), committed.Generation)
	return env, a, b, dlA, id, committed
}

// T3: the stale worker wakes up after the winner completed. Every ownership
// call it makes is fenced off and the winner's result stays untouched. This is
// the integrated version of
// internal/task/sqlite.TestFencing_StaleWorkerCannotTouchWinnersResult.
func TestCrashRecovery_T3_StaleWorkerIsFenced(t *testing.T) {
	_, a, b, dlA, id, committed := stageTakeover(t)
	ctx := context.Background()

	// A is back on the network.
	a.gate.down.Store(false)
	const staleGen = 1
	_, renewErr := a.gate.RenewLease(ctx, id, "worker-a", staleGen, events.LeaseUpdate{})
	stale := events.AttemptResult{
		FileName:    "stale.bin",
		StorageType: committed.StorageType,
		StorageKey:  strconv.FormatUint(id, 10) + "/1/stale-0011223344556677.bin",
		Checksum:    &events.ChecksumInfo{ChecksumType: "md5", ChecksumValue: "stale"},
	}
	for name, err := range map[string]error{
		"renew":    renewErr,
		"complete": a.gate.CompleteAttempt(ctx, id, "worker-a", staleGen, stale),
		"fail":     a.gate.FailAttempt(ctx, id, "worker-a", staleGen, errors.New("stale failure")),
		"release":  a.gate.ReleaseAttempt(ctx, id, "worker-a", staleGen),
	} {
		require.ErrorIs(t, err, events.ErrAttemptRejected, name)
	}
	_, err := a.gate.ClaimTask(ctx, id, "worker-a")
	require.ErrorIs(t, err, events.ErrTaskNotClaimable, "it cannot claim the completed task again")
	require.Equal(t, committed, b.state(t, id), "B's result must be untouched")

	// Now A's own download resumes and A runs its real code path against the
	// task service. Its storing-step renewal is fenced and it stops without
	// forcing a result.
	rejectedBefore := a.gate.rejected("RenewLease")
	close(dlA.hold)
	require.Eventually(t, func() bool { return a.gate.rejected("RenewLease") > rejectedBefore },
		5*time.Second, 10*time.Millisecond, "A's own storing renewal was never fenced")
	a.stopAll()

	require.Len(t, a.gate.forwardedCalls("CompleteAttempt"), 1,
		"A's own flow must not try to commit (only the explicit stale call above reached the service)")
	require.Equal(t, committed, b.state(t, id), "B's result must still be untouched")
	require.Equal(t, 1, b.outboxRows(t, id, task.OutboxEventTaskCompleted), "exactly one completion event: B's")
	require.Equal(t, []string{committed.StoragePath}, b.objects(t, id),
		"only B's object remains; A's stale object is removed")
}

// T3b: stale notifications change nothing. After B won, A's late FailAttempt
// and raw task.failed / task.progress.updated / task.completed events on the
// bus must not alter B's committed state: the lifecycle consumer only
// acknowledges them and ownership calls are fenced.
func TestCrashRecovery_T3b_StaleEventsChangeNothing(t *testing.T) {
	_, a, b, dlA, id, committed := stageTakeover(t)
	ctx := context.Background()
	t.Cleanup(func() {
		select {
		case <-dlA.hold:
		default:
			close(dlA.hold)
		}
	})

	a.gate.down.Store(false)
	require.ErrorIs(t, a.gate.FailAttempt(ctx, id, "worker-a", 1, errors.New("late failure")), events.ErrAttemptRejected)
	_, err := a.gate.RenewLease(ctx, id, "worker-a", 1, events.LeaseUpdate{
		Progress: &events.AttemptProgress{Progress: 3, DownloadedBytes: 3, TotalBytes: 100},
	})
	require.ErrorIs(t, err, events.ErrAttemptRejected)

	publish := func(topic string, payload any) {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, b.pub.Publish(topic, message.NewMessage("raw-"+topic, raw)))
	}
	now := time.Now().UTC()
	publish("task.failed", events.TaskFailedEvent{TaskID: id, Error: "late failure", FailedAt: now})
	publish("task.progress.updated", events.TaskProgressUpdatedEvent{
		TaskID: id, Progress: 3, DownloadedBytes: 3, TotalBytes: 100, UpdatedAt: now,
	})
	publish("task.completed", events.TaskCompletedEvent{
		TaskID: id, FileName: "stale.bin", StorageType: string(committed.StorageType),
		StorageKey: strconv.FormatUint(id, 10) + "/1/stale-0011223344556677.bin", CompletedAt: now,
	})

	require.Never(t, func() bool {
		return b.state(t, id).StoragePath != committed.StoragePath ||
			b.state(t, id).Status != task.StatusCompleted
	}, 400*time.Millisecond, 20*time.Millisecond)
	require.Equal(t, committed, b.state(t, id), "stale events must not alter B's committed state")
	require.Equal(t, 1, b.outboxRows(t, id, task.OutboxEventTaskCompleted), "still the single completion event")
}

// T4: CompleteAttempt commits but its response is lost; the same generation
// retries the identical result and gets an idempotent success. No second
// download, one completion event.
func TestCrashRecovery_T4_LostCompletionResponse(t *testing.T) {
	env := newCrashEnv(t)
	dl := &scriptedDownloader{body: "t4 payload"}
	p := startPipeline(t, dl, env.opts("worker-a"))
	p.gate.loseNextComplete.Store(true)
	// By the retry the lease is long gone: the idempotent path must not care.
	p.gate.afterLost = func() { env.clock.Advance(time.Hour) }

	id := p.create(t)
	done := p.waitFor(t, id, task.StatusCompleted)

	require.Eventually(t, func() bool { return len(p.gate.forwardedCalls("CompleteAttempt")) >= 2 },
		10*time.Second, 10*time.Millisecond, "the worker never retried the completion")
	p.waitOutboxDrained(t)

	calls := p.gate.forwardedCalls("CompleteAttempt")
	require.Len(t, calls, 2, "the lost response is retried exactly once")
	require.NoError(t, calls[0].Err, "the first call committed (its response was replaced by an error)")
	require.NoError(t, calls[1].Err, "the retry of the same generation and result is an idempotent success")

	require.Equal(t, uint64(1), done.Generation)
	require.Equal(t, done, p.state(t, id), "the retry must not change the committed result")
	require.Equal(t, int32(1), dl.starts.Load(), "no second download")
	require.Equal(t, 1, p.outboxRows(t, id, task.OutboxEventTaskCompleted), "one completion event")
	require.Equal(t, []string{done.StoragePath}, p.objects(t, id), "the committed object was kept")
	require.Zero(t, p.gate.count("ReleaseAttempt", false)+p.gate.count("FailAttempt", false))
}
