package download

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	apperrors "github.com/yuisofull/goload/internal/errors"
	"github.com/yuisofull/goload/internal/events"
	"github.com/yuisofull/goload/internal/storage"
)

func TestGenerateStorageKeyPrefersResolvedFileNameExtension(t *testing.T) {
	svc := &service{}
	key := svc.generateStorageKey(TaskRequest{
		TaskID:    1,
		FileName:  "cjrCRgMpxacNCRvmjfgpfwGR",
		SourceURL: "https://example.com/download/cjrCRgMpxacNCRvmjfgpfwGR?token=abc",
	}, "design_rationale_example_1.pdf")

	if !strings.HasPrefix(key, "1/design_rationale_example_1-") {
		t.Fatalf("expected key to use resolved filename, got %q", key)
	}
	if filepath.Ext(key) != ".pdf" {
		t.Fatalf("expected key to preserve .pdf extension, got %q", key)
	}
}

func TestGenerateStorageKeyFallsBackToURLPathWithoutQuery(t *testing.T) {
	svc := &service{}
	key := svc.generateStorageKey(TaskRequest{
		TaskID:    7,
		SourceURL: "https://example.com/files/archive.tar.gz?signature=abc",
	}, "")

	if !strings.HasPrefix(key, "7/archive.tar-") {
		t.Fatalf("expected key to use URL path filename, got %q", key)
	}
	if filepath.Ext(key) != ".gz" {
		t.Fatalf("expected key to preserve .gz extension, got %q", key)
	}
}

type fakeDownloader struct {
	downloads atomic.Int32
}

func (d *fakeDownloader) GetFileInfo(ctx context.Context, rawURL string, auth *AuthConfig) (*FileMetadata, error) {
	return &FileMetadata{
		FileName:    "file.txt",
		FileSize:    int64(len("content")),
		ContentType: "text/plain",
	}, nil
}

func (d *fakeDownloader) Download(
	ctx context.Context,
	rawURL string,
	auth *AuthConfig,
	opts DownloadOptions,
) (io.ReadCloser, int64, error) {
	d.downloads.Add(1)
	return io.NopCloser(strings.NewReader("content")), int64(len("content")), nil
}

func (d *fakeDownloader) SupportsResume() bool { return false }

type fakeStorage struct{}

func (s *fakeStorage) Store(ctx context.Context, key string, reader io.Reader, metadata *storage.FileMetadata) error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}
	return nil
}

func (s *fakeStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *fakeStorage) GetWithRange(ctx context.Context, key string, start, end int64) (io.ReadCloser, error) {
	return nil, nil
}

func (s *fakeStorage) GetInfo(ctx context.Context, key string) (*storage.FileMetadata, error) {
	return nil, nil
}

func (s *fakeStorage) Exists(ctx context.Context, key string) (bool, error) { return false, nil }
func (s *fakeStorage) Delete(ctx context.Context, key string) error         { return nil }

type fakePublisher struct {
	mu        sync.Mutex
	completed *events.TaskCompletedEvent
	failed    *events.TaskFailedEvent
	statusErr error
}

func (p *fakePublisher) PublishTaskStatusUpdated(ctx context.Context, event events.TaskStatusUpdatedEvent) error {
	return p.statusErr
}

func (p *fakePublisher) PublishTaskProgressUpdated(ctx context.Context, event events.TaskProgressUpdatedEvent) error {
	return nil
}

func (p *fakePublisher) PublishTaskCompleted(ctx context.Context, event events.TaskCompletedEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed = &event
	return nil
}

func (p *fakePublisher) PublishTaskFailed(ctx context.Context, event events.TaskFailedEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failed = &event
	return nil
}

func TestExecuteTaskAttemptsDownloadWhenMaxRetriesZero(t *testing.T) {
	synctest.Test(t, testExecuteTaskAttemptsDownloadWhenMaxRetriesZero)
}

func testExecuteTaskAttemptsDownloadWhenMaxRetriesZero(t *testing.T) {
	store := &fakeStorage{}
	pub := &fakePublisher{}
	dl := &fakeDownloader{}
	svc := NewService(store, pub, WithStorageType(storage.TypeMinio))
	svc.RegisterDownloader("HTTP", dl)

	err := svc.ExecuteTask(context.Background(), TaskRequest{
		TaskID:     11,
		SourceURL:  "https://example.com/file.txt",
		SourceType: "HTTP",
		DownloadOptions: &DownloadOptions{
			MaxRetries: 0,
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v", err)
	}
	synctest.Wait()
	if dl.downloads.Load() != 1 {
		t.Fatalf("expected one download attempt, got %d", dl.downloads.Load())
	}
	if pub.completed == nil {
		t.Fatal("expected completion event")
	}
	if pub.completed.StorageType != storage.TypeMinio.String() {
		t.Fatalf("expected storage type %q, got %q", storage.TypeMinio, pub.completed.StorageType)
	}
}

type blockingDownloader struct {
	fakeDownloader
	started chan struct{}
	release <-chan struct{}
}

func (d *blockingDownloader) Download(
	ctx context.Context,
	rawURL string,
	auth *AuthConfig,
	opts DownloadOptions,
) (io.ReadCloser, int64, error) {
	d.started <- struct{}{}
	select {
	case <-d.release:
		return d.fakeDownloader.Download(ctx, rawURL, auth, opts)
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}
}

func TestExecuteTaskConcurrencyLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
	}{
		{name: "one at a time", limit: 1},
		{name: "concurrent downloads", limit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				release := make(chan struct{})
				dl := &blockingDownloader{started: make(chan struct{}, tc.limit+1), release: release}
				svc := NewService(&fakeStorage{}, &fakePublisher{}, WithMaxConcurrent(tc.limit))
				svc.RegisterDownloader("HTTP", dl)

				for taskID := 1; taskID <= tc.limit; taskID++ {
					if err := svc.ExecuteTask(ctx, TaskRequest{TaskID: uint64(taskID), SourceType: "HTTP"}); err != nil {
						t.Fatalf("ExecuteTask() error = %v", err)
					}
				}
				synctest.Wait()
				if got := len(dl.started); got != tc.limit {
					t.Fatalf("started %d downloads, want %d", got, tc.limit)
				}

				// A duplicate must be rejected even while all download slots are busy.
				err := svc.ExecuteTask(ctx, TaskRequest{TaskID: 1, SourceType: "HTTP"})
				if !apperrors.IsError(err, apperrors.ErrCodeConflict) {
					t.Fatalf("duplicate ExecuteTask() error = %v, want conflict", err)
				}

				accepted := make(chan error, 1)
				go func() {
					req := TaskRequest{TaskID: uint64(tc.limit + 1), SourceType: "HTTP"}
					accepted <- svc.ExecuteTask(ctx, req)
				}()
				synctest.Wait()
				select {
				case err := <-accepted:
					t.Fatalf("ExecuteTask() returned while all slots were busy: %v", err)
				default:
				}
				if got := len(dl.started); got != tc.limit {
					t.Fatalf("concurrency limit exceeded: started %d downloads", got)
				}

				release <- struct{}{}
				synctest.Wait()
				if err := <-accepted; err != nil {
					t.Fatalf("waiting ExecuteTask() error = %v", err)
				}
				if got := len(dl.started); got != tc.limit+1 {
					t.Fatalf("started %d downloads after freeing a slot, want %d", got, tc.limit+1)
				}

				close(release)
				synctest.Wait()
				if got := svc.GetActiveTaskCount(ctx); got != 0 {
					t.Fatalf("active tasks after completion = %d, want 0", got)
				}
			})
		})
	}
}

func TestExecuteTaskCancellationWhileWaitingForSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		release := make(chan struct{})
		dl := &blockingDownloader{started: make(chan struct{}, 2), release: release}
		svc := NewService(&fakeStorage{}, &fakePublisher{}, WithMaxConcurrent(1))
		svc.RegisterDownloader("HTTP", dl)
		if err := svc.ExecuteTask(ctx, TaskRequest{TaskID: 1, SourceType: "HTTP"}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()

		waitingCtx, cancelWaiting := context.WithCancel(ctx)
		defer cancelWaiting()
		result := make(chan error, 1)
		go func() {
			result <- svc.ExecuteTask(waitingCtx, TaskRequest{TaskID: 2, SourceType: "HTTP"})
		}()
		synctest.Wait()
		cancelWaiting()
		synctest.Wait()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("ExecuteTask() error = %v, want context cancellation", err)
		}
		if got := svc.GetActiveTaskCount(ctx); got != 1 {
			t.Fatalf("active tasks after cancelling waiting task = %d, want 1", got)
		}
		if got := len(dl.started); got != 1 {
			t.Fatalf("started %d downloads, want 1", got)
		}
		close(release)
		synctest.Wait()
		if got := svc.GetActiveTaskCount(ctx); got != 0 {
			t.Fatalf("active tasks after completion = %d, want 0", got)
		}
	})
}

func TestExecuteTaskReportsAsynchronousFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pub := &fakePublisher{statusErr: errors.New("publisher unavailable")}
		reported := make(chan error, 1)
		svc := NewService(&fakeStorage{}, pub,
			WithMaxConcurrent(1),
			WithErrorHandler(func(_ context.Context, err error) { reported <- err }),
		)
		svc.RegisterDownloader("HTTP", &fakeDownloader{})
		if err := svc.ExecuteTask(t.Context(), TaskRequest{TaskID: 1, SourceType: "HTTP"}); err != nil {
			t.Fatalf("ExecuteTask() error = %v", err)
		}
		synctest.Wait()
		if pub.failed == nil || pub.failed.TaskID != 1 {
			t.Fatalf("expected task failure event, got %v", pub.failed)
		}
		if len(reported) == 0 {
			t.Fatal("expected error handler to report asynchronous failure")
		}
		if got := svc.GetActiveTaskCount(t.Context()); got != 0 {
			t.Fatalf("active tasks after failure = %d, want 0", got)
		}

		// A failed task must release its slot for the next download.
		pub.statusErr = nil
		if err := svc.ExecuteTask(t.Context(), TaskRequest{TaskID: 2, SourceType: "HTTP"}); err != nil {
			t.Fatalf("ExecuteTask() after failure error = %v", err)
		}
		synctest.Wait()
		if pub.completed == nil || pub.completed.TaskID != 2 {
			t.Fatalf("expected next task to complete, got %v", pub.completed)
		}
	})
}

func TestPausableProgressReader_ConcurrentPauseAndRead(t *testing.T) {
	reader := NewPausableProgressReader(strings.NewReader(strings.Repeat("x", 1<<20)), func(int64) {})
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			_, err := reader.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	for range 1000 {
		reader.Pause()
		reader.Resume()
	}
	reader.Resume()
	<-done
}

func TestGenerateStorageKeyNamespacesByGeneration(t *testing.T) {
	svc := &service{}
	req := TaskRequest{TaskID: 9, SourceURL: "https://example.com/files/file.txt"}

	legacy := svc.generateStorageKey(req, "file.txt")
	req.Generation = 1
	first := svc.generateStorageKey(req, "file.txt")
	req.Generation = 2
	second := svc.generateStorageKey(req, "file.txt")

	if !strings.HasPrefix(legacy, "9/file-") {
		t.Fatalf("an unclaimed request must keep the legacy key layout, got %q", legacy)
	}
	if !strings.HasPrefix(first, "9/1/file-") {
		t.Fatalf("generation 1 key must be namespaced by generation, got %q", first)
	}
	if !strings.HasPrefix(second, "9/2/file-") {
		t.Fatalf("generation 2 key must be namespaced by generation, got %q", second)
	}
	if filepath.Ext(second) != ".txt" {
		t.Fatalf("expected key to preserve .txt extension, got %q", second)
	}
	if strings.TrimPrefix(first, "9/1/") != strings.TrimPrefix(legacy, "9/") {
		t.Fatalf("only the generation segment may differ: %q vs %q", first, legacy)
	}
}

// keyedStorage records stored objects by key. failNextStore makes the next
// Store fail after it has consumed the reader.
type keyedStorage struct {
	fakeStorage
	mu       sync.Mutex
	objects  map[string]bool
	deleted  []string
	failNext bool
}

func (s *keyedStorage) Store(ctx context.Context, key string, reader io.Reader, metadata *storage.FileMetadata) error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failNext {
		s.failNext = false
		return errors.New("disk full")
	}
	s.objects[key] = true
	return nil
}

func (s *keyedStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	delete(s.objects, key)
	return nil
}

func (s *keyedStorage) failNextStore() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNext = true
}

func (s *keyedStorage) has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.objects[key]
}

func (s *keyedStorage) deletedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.deleted...)
}

func TestExecuteTask_StaleAttemptCleanupKeepsWinnersObject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &keyedStorage{objects: map[string]bool{}}
		svc := NewService(store, &fakePublisher{})
		svc.RegisterDownloader("HTTP", &fakeDownloader{})
		base := TaskRequest{TaskID: 9, SourceURL: "https://example.com/files/file.txt", SourceType: "HTTP"}

		// The replacement attempt (generation 2) stores its object.
		winner := base
		winner.Generation = 2
		if err := svc.ExecuteTask(t.Context(), winner); err != nil {
			t.Fatalf("ExecuteTask(winner) error = %v", err)
		}
		synctest.Wait()
		winnerKey := svc.generateStorageKey(winner, "file.txt")
		if !store.has(winnerKey) {
			t.Fatalf("winner object %q was not stored", winnerKey)
		}

		// The stale attempt (generation 1) then fails while storing and
		// cleans up after itself.
		stale := base
		stale.Generation = 1
		staleKey := svc.generateStorageKey(stale, "file.txt")
		store.failNextStore()
		if err := svc.ExecuteTask(t.Context(), stale); err != nil {
			t.Fatalf("ExecuteTask(stale) error = %v", err)
		}
		synctest.Wait()

		if got := store.deletedKeys(); len(got) != 1 || got[0] != staleKey {
			t.Fatalf("deleted keys = %v, want only the stale attempt's own key %q", got, staleKey)
		}
		if !store.has(winnerKey) {
			t.Fatal("the stale attempt's cleanup deleted the winner's object")
		}
	})
}

func TestPauseTaskStopsExecutionWithoutReportingFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		dl := &blockingDownloader{started: make(chan struct{}, 2), release: release}
		pub := &fakePublisher{}
		svc := NewService(&fakeStorage{}, pub, WithMaxConcurrent(1))
		svc.RegisterDownloader("HTTP", dl)
		req := TaskRequest{TaskID: 1, SourceType: "HTTP"}
		if err := svc.ExecuteTask(t.Context(), req); err != nil {
			t.Fatalf("ExecuteTask() error = %v", err)
		}
		synctest.Wait()

		if err := svc.PauseTask(t.Context(), 1); err != nil {
			t.Fatalf("PauseTask() error = %v", err)
		}
		synctest.Wait()

		if got := svc.GetActiveTaskCount(t.Context()); got != 0 {
			t.Fatalf("active tasks after pause = %d, want 0: a paused download must not stay parked", got)
		}
		if pub.failed != nil {
			t.Fatalf("a paused download must not report a failure, got %+v", pub.failed)
		}

		// A resume is a fresh dispatch of the same task. It must start a new
		// download instead of being rejected as already running.
		if err := svc.ExecuteTask(t.Context(), req); err != nil {
			t.Fatalf("ExecuteTask() after pause error = %v", err)
		}
		synctest.Wait()
		if got := len(dl.started); got != 2 {
			t.Fatalf("started %d downloads, want 2", got)
		}

		close(release)
		synctest.Wait()
		if pub.completed == nil || pub.completed.TaskID != 1 {
			t.Fatalf("expected the fresh download to complete, got %v", pub.completed)
		}
	})
}

func TestCancelTaskStopsQueuedExecution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		dl := &blockingDownloader{started: make(chan struct{}, 2), release: release}
		pub := &fakePublisher{}
		svc := NewService(&fakeStorage{}, pub, WithMaxConcurrent(1))
		svc.RegisterDownloader("HTTP", dl)
		if err := svc.ExecuteTask(t.Context(), TaskRequest{TaskID: 1, SourceType: "HTTP"}); err != nil {
			t.Fatalf("ExecuteTask() error = %v", err)
		}
		synctest.Wait()

		// Task 2 waits for the only slot.
		queued := make(chan error, 1)
		go func() {
			queued <- svc.ExecuteTask(t.Context(), TaskRequest{TaskID: 2, SourceType: "HTTP"})
		}()
		synctest.Wait()

		if err := svc.CancelTask(t.Context(), 2); err != nil {
			t.Fatalf("CancelTask() error = %v", err)
		}
		synctest.Wait()
		if err := <-queued; err != nil {
			t.Fatalf("a cancelled queued task must be dropped, not redelivered: ExecuteTask() error = %v", err)
		}
		if got := len(dl.started); got != 1 {
			t.Fatalf("started %d downloads, want 1: the cancelled task must never start", got)
		}
		if pub.failed != nil {
			t.Fatalf("a cancelled task must not report a failure, got %+v", pub.failed)
		}

		close(release)
		synctest.Wait()
		if got := svc.GetActiveTaskCount(t.Context()); got != 0 {
			t.Fatalf("active tasks after completion = %d, want 0", got)
		}
	})
}
