package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/internal/download"
	downloadtransport "github.com/yuisofull/goload/internal/download/transport"
	"github.com/yuisofull/goload/internal/storage"
	"github.com/yuisofull/goload/internal/task"
	tasksqlite "github.com/yuisofull/goload/internal/task/sqlite"
	tasktransport "github.com/yuisofull/goload/internal/task/transport"
	"github.com/yuisofull/goload/pkg/message/inmem"
)

// scriptedDownloader serves a fixed body. Its first failInfo GetFileInfo calls
// fail, which fails those attempts as a whole before any download starts.
type scriptedDownloader struct {
	body     string
	failInfo atomic.Int32
	starts   atomic.Int32
}

func (d *scriptedDownloader) GetFileInfo(
	context.Context,
	string,
	*download.AuthConfig,
) (*download.FileMetadata, error) {
	if d.failInfo.Add(-1) >= 0 {
		return nil, errors.New("source unavailable")
	}
	return &download.FileMetadata{
		FileName:    "file.bin",
		FileSize:    int64(len(d.body)),
		ContentType: "application/octet-stream",
	}, nil
}

func (d *scriptedDownloader) Download(
	context.Context,
	string,
	*download.AuthConfig,
	download.DownloadOptions,
) (io.ReadCloser, int64, error) {
	d.starts.Add(1)
	return io.NopCloser(strings.NewReader(d.body)), int64(len(d.body)), nil
}

func (d *scriptedDownloader) SupportsResume() bool { return false }

// pocketPipeline is the background half of pocket mode, wired the way main
// wires it: SQLite, the in-memory bus, the outbox relay, the notification-only
// lifecycle consumer, the attempt-aware download worker and the reconciler.
// Only the intervals are shortened.
type pocketPipeline struct {
	tasks    task.Service
	attempts task.AttemptRepository
	outbox   task.OutboxRepository
	store    *storage.Local
}

func startPocketPipeline(t *testing.T, dl download.Downloader) *pocketPipeline {
	t.Helper()

	pool, err := sqlitex.Open(filepath.Join(t.TempDir(), "pocket_pipeline.db"), 0, 10)
	require.NoError(t, err)
	require.NoError(t, runMigrations(pool))
	store, err := storage.NewLocalBackend(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)

	logger := log.NewNopLogger()
	pub, sub := inmem.NewPublisherAndSubscriber(logger)

	taskRepo := tasksqlite.NewTaskRepo(pool)
	tx := tasksqlite.NewTxManager(pool)
	outboxRepo := tasksqlite.NewOutboxRepo(pool)
	attemptRepo := tasksqlite.NewAttemptRepo(pool)
	reconcileRepo := tasksqlite.NewReconcileRepo(pool)

	taskPub := task.NewEventPublisher(pub)
	relay := task.NewOutboxRelay(outboxRepo, taskPub, task.WithOutboxPollInterval(10*time.Millisecond))
	taskSvc := task.NewService(taskRepo, *taskPub, tx, outboxRepo, attemptRepo)
	attemptSvc := task.NewAttemptService(attemptRepo, tx, outboxRepo,
		task.WithRetryBackoff(func(int) time.Duration { return 20 * time.Millisecond }))
	reconciler := task.NewReconciler(reconcileRepo, taskRepo, outboxRepo,
		task.WithReconcileInterval(20*time.Millisecond))
	lifecycle := tasktransport.NewEventConsumer(taskSvc, sub, func(context.Context, error) {})

	dlSvc := download.NewService(
		store,
		download.NewDownloadEventPublisher(pub),
		download.WithStorageType(storage.TypeLocal),
		download.WithAttempts(attemptSvc, "worker-test"),
	)
	dlSvc.RegisterDownloader("HTTPS", dl)
	worker := downloadtransport.NewEventConsumer(dlSvc, sub, logger, downloadtransport.WithAttemptRunner(dlSvc))

	ctx, cancel := context.WithCancel(context.Background())
	var running sync.WaitGroup
	for _, run := range []func(context.Context) error{lifecycle.Start, worker.Start, relay.Run, reconciler.Run} {
		running.Add(1)
		go func() {
			defer running.Done()
			_ = run(ctx)
		}()
	}
	t.Cleanup(func() {
		cancel()
		running.Wait()
		_ = store.Close()
		_ = pool.Close()
	})

	return &pocketPipeline{tasks: taskSvc, attempts: attemptRepo, outbox: outboxRepo, store: store}
}

// create stores a task the way the HTTP API does.
func (p *pocketPipeline) create(t *testing.T) uint64 {
	t.Helper()

	created, err := p.tasks.CreateTask(context.Background(), &task.CreateTaskParam{
		OfAccountID: 7,
		FileName:    "file.bin",
		SourceURL:   "https://example.test/file.bin",
	})
	require.NoError(t, err)
	return created.ID
}

func (p *pocketPipeline) state(t *testing.T, taskID uint64) *task.AttemptState {
	t.Helper()

	st, err := p.attempts.GetAttemptState(context.Background(), taskID)
	require.NoError(t, err)
	return st
}

// waitFor waits until the task reaches status and returns its state.
func (p *pocketPipeline) waitFor(t *testing.T, taskID uint64, status task.TaskStatus) *task.AttemptState {
	t.Helper()

	require.Eventually(t, func() bool {
		st, err := p.attempts.GetAttemptState(context.Background(), taskID)
		return err == nil && st.Status == status
	}, 10*time.Second, 10*time.Millisecond, "task %d never reached %s", taskID, status)
	return p.state(t, taskID)
}

// Happy path: dispatch -> claim -> complete -> lifecycle completion event
// published through the relay.
func TestPocketPipeline_DispatchClaimCompleteAndNotify(t *testing.T) {
	dl := &scriptedDownloader{body: "pocket payload"}
	p := startPocketPipeline(t, dl)
	ctx := context.Background()

	id := p.create(t)
	done := p.waitFor(t, id, task.StatusCompleted)

	require.Equal(t, uint64(1), done.Generation, "the first claim is generation 1")
	require.Empty(t, done.OwnerID)
	require.Nil(t, done.LeaseExpiresAt)
	require.Zero(t, done.RetryCount)
	require.Equal(t, storage.TypeLocal, done.StorageType)
	require.True(t, strings.HasPrefix(done.StoragePath, strconv.FormatUint(id, 10)+"/1/file-"),
		"the result is the attempt's own generation-scoped key, got %q", done.StoragePath)
	sum := md5.Sum([]byte(dl.body))
	wantChecksum := &task.ChecksumInfo{ChecksumType: "md5", ChecksumValue: hex.EncodeToString(sum[:])}
	require.Equal(t, wantChecksum, done.Checksum)
	require.Equal(t, task.DownloadProgress{Progress: 100, DownloadedBytes: 14, TotalBytes: 14}, done.Progress)
	require.Equal(t, int32(1), dl.starts.Load())

	stored, err := p.store.Get(ctx, done.StoragePath)
	require.NoError(t, err)
	body, err := io.ReadAll(stored)
	require.NoError(t, stored.Close())
	require.NoError(t, err)
	require.Equal(t, dl.body, string(body))

	// CompleteAttempt wrote the task.completed row with the result. The relay
	// publishes it because the lifecycle consumer is subscribed, and the
	// consumer only acknowledges it: the committed row stays as it is.
	require.Eventually(t, func() bool {
		unsent, err := p.outbox.FetchUnsent(ctx, task.OutboxCursor{}, 10)
		return err == nil && len(unsent) == 0
	}, 5*time.Second, 10*time.Millisecond, "the outbox never drained")
	require.Equal(t, done, p.state(t, id))
}

// A failed attempt goes back to PENDING with a retry time. Nothing but the
// reconciler dispatches it again.
func TestPocketPipeline_FailedAttemptIsRetriedByTheReconciler(t *testing.T) {
	dl := &scriptedDownloader{body: "second time lucky"}
	dl.failInfo.Store(1)
	p := startPocketPipeline(t, dl)

	id := p.create(t)
	done := p.waitFor(t, id, task.StatusCompleted)

	require.Equal(t, uint64(2), done.Generation, "the retry is a new claim")
	require.Equal(t, 1, done.RetryCount)
	require.Nil(t, done.ErrorMessage, "the completion clears the failure message")
	require.True(t, strings.HasPrefix(done.StoragePath, strconv.FormatUint(id, 10)+"/2/file-"),
		"the result is the second attempt's key, got %q", done.StoragePath)
	require.Equal(t, int32(1), dl.starts.Load(), "the failed attempt never started a download")
}
