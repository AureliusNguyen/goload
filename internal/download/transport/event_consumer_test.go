package downloadtransport

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/internal/download"
	apperrors "github.com/yuisofull/goload/internal/errors"
	"github.com/yuisofull/goload/internal/events"
	"github.com/yuisofull/goload/pkg/message"
	"github.com/yuisofull/goload/pkg/message/inmem"
)

type fakeDownloadService struct {
	executeFn func(ctx context.Context, req download.TaskRequest) error
	pauseFn   func(ctx context.Context, taskID uint64) error
}

func (f *fakeDownloadService) ExecuteTask(ctx context.Context, req download.TaskRequest) error {
	if f.executeFn != nil {
		return f.executeFn(ctx, req)
	}
	return nil
}

func (f *fakeDownloadService) PauseTask(ctx context.Context, taskID uint64) error {
	if f.pauseFn != nil {
		return f.pauseFn(ctx, taskID)
	}
	return nil
}

func (f *fakeDownloadService) CancelTask(_ context.Context, _ uint64) error {
	return nil
}

func (f *fakeDownloadService) StreamFile(
	_ context.Context,
	_ download.FileStreamRequest,
) (*download.FileStreamResponse, error) {
	return nil, apperrors.ErrInternal
}

func (f *fakeDownloadService) GetActiveTaskCount(_ context.Context) int { return 0 }

func waitAckOrNack(t *testing.T, msg *message.Message) bool {
	t.Helper()
	select {
	case <-msg.Acked():
		return true
	case <-msg.Nacked():
		return false
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ack/nack")
		return false
	}
}

func TestProcessTaskCreatedEvents_AcksAfterExecuteTask(t *testing.T) {
	t.Parallel()

	executed := make(chan download.TaskRequest, 1)
	ec := NewEventConsumer(&fakeDownloadService{
		executeFn: func(_ context.Context, req download.TaskRequest) error {
			executed <- req
			return nil
		},
	}, nil, log.NewNopLogger())

	ch := make(chan *message.Message, 1)
	payload, err := json.Marshal(events.TaskCreatedEvent{TaskID: 42, SourceType: "HTTP"})
	require.NoError(t, err)
	msg := message.NewMessage("created-1", payload)
	ch <- msg
	close(ch)

	ec.processTaskCreatedEvents(t.Context(), ch)

	select {
	case req := <-executed:
		assert.Equal(t, uint64(42), req.TaskID)
	default:
		t.Fatal("ExecuteTask was not called")
	}
	assert.True(t, waitAckOrNack(t, msg))
}

func TestProcessTaskCreatedEvents_NacksTransientFailure(t *testing.T) {
	t.Parallel()

	ec := NewEventConsumer(&fakeDownloadService{
		executeFn: func(_ context.Context, _ download.TaskRequest) error {
			return &apperrors.Error{Code: apperrors.ErrCodeInternal, Message: "disk full"}
		},
	}, nil, log.NewNopLogger())

	ch := make(chan *message.Message, 1)
	payload, err := json.Marshal(events.TaskCreatedEvent{TaskID: 7, SourceType: "HTTP"})
	require.NoError(t, err)
	msg := message.NewMessage("created-fail", payload)
	ch <- msg
	close(ch)

	ec.processTaskCreatedEvents(t.Context(), ch)
	assert.False(t, waitAckOrNack(t, msg))
}

func TestProcessTaskPausedEvents_AcksNotFound(t *testing.T) {
	t.Parallel()

	ec := NewEventConsumer(&fakeDownloadService{
		pauseFn: func(_ context.Context, _ uint64) error {
			return &apperrors.Error{Code: apperrors.ErrCodeNotFound, Message: "task not found in active tasks"}
		},
	}, nil, log.NewNopLogger())

	ch := make(chan *message.Message, 1)
	payload, err := json.Marshal(events.TaskPausedEvent{TaskID: 9})
	require.NoError(t, err)
	msg := message.NewMessage("paused-1", payload)
	ch <- msg
	close(ch)

	ec.processTaskPausedEvents(t.Context(), ch)
	assert.True(t, waitAckOrNack(t, msg))
}

func TestProcessTaskPausedEvents_NacksOtherErrors(t *testing.T) {
	t.Parallel()

	ec := NewEventConsumer(&fakeDownloadService{
		pauseFn: func(_ context.Context, _ uint64) error {
			return &apperrors.Error{Code: apperrors.ErrCodeInternal, Message: "pause failed"}
		},
	}, nil, log.NewNopLogger())

	ch := make(chan *message.Message, 1)
	payload, err := json.Marshal(events.TaskPausedEvent{TaskID: 9})
	require.NoError(t, err)
	msg := message.NewMessage("paused-fail", payload)
	ch <- msg
	close(ch)

	ec.processTaskPausedEvents(t.Context(), ch)
	assert.False(t, waitAckOrNack(t, msg))
}

// recordingSubscriber records the topics a consumer subscribes to. Its
// channels deliver nothing and close with the subscription context.
type recordingSubscriber struct {
	mu     sync.Mutex
	topics []string
}

func (s *recordingSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	s.mu.Lock()
	s.topics = append(s.topics, topic)
	s.mu.Unlock()

	ch := make(chan *message.Message)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func (s *recordingSubscriber) Close() error { return nil }

func (s *recordingSubscriber) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.topics...)
}

func TestStart_SubscribesToDispatchAndStopTopicsOnly(t *testing.T) {
	t.Parallel()

	sub := &recordingSubscriber{}
	ec := NewEventConsumer(&fakeDownloadService{}, sub, log.NewNopLogger())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- ec.Start(ctx) }()

	require.Eventually(t, func() bool { return len(sub.snapshot()) >= 3 }, 2*time.Second, 5*time.Millisecond)
	cancel()
	require.ErrorIs(t, <-stopped, context.Canceled)

	// A resume is a fresh task.created dispatch; there is no task.resumed.
	assert.Equal(t, []string{"task.created", "task.paused", "task.cancelled"}, sub.snapshot())
}

// fakeAttemptRunner is a scripted download.AttemptRunner.
type fakeAttemptRunner struct {
	runFn func(ctx context.Context, req download.TaskRequest) error
}

func (f *fakeAttemptRunner) RunAttempt(ctx context.Context, req download.TaskRequest) error {
	return f.runFn(ctx, req)
}

// createdMessage is a task.created delivery for taskID.
func createdMessage(t *testing.T, taskID uint64) *message.Message {
	t.Helper()
	payload, err := json.Marshal(events.TaskCreatedEvent{TaskID: taskID, SourceType: "HTTP"})
	require.NoError(t, err)
	return message.NewMessage("created", payload)
}

func TestProcessTaskCreatedEvents_AttemptPathAcksOnlyAfterTheOutcomeIsSettled(t *testing.T) {
	t.Parallel()

	var legacyCalls atomic.Int32
	started := make(chan download.TaskRequest, 1)
	settle := make(chan error)
	ec := NewEventConsumer(
		&fakeDownloadService{executeFn: func(context.Context, download.TaskRequest) error {
			legacyCalls.Add(1)
			return nil
		}},
		nil,
		log.NewNopLogger(),
		WithAttemptRunner(&fakeAttemptRunner{runFn: func(_ context.Context, req download.TaskRequest) error {
			started <- req
			return <-settle
		}}),
	)

	ch := make(chan *message.Message, 1)
	msg := createdMessage(t, 42)
	ch <- msg
	close(ch)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ec.processTaskCreatedEvents(t.Context(), ch)
	}()

	select {
	case req := <-started:
		assert.Equal(t, uint64(42), req.TaskID)
	case <-time.After(2 * time.Second):
		t.Fatal("RunAttempt was not called")
	}
	select {
	case <-msg.Acked():
		t.Fatal("the delivery was acknowledged while its attempt was still running")
	case <-msg.Nacked():
		t.Fatal("the delivery was negatively acknowledged while its attempt was still running")
	case <-time.After(50 * time.Millisecond):
	}

	settle <- nil
	assert.True(t, waitAckOrNack(t, msg), "a settled attempt acknowledges its delivery")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processTaskCreatedEvents did not return after the attempt settled")
	}
	assert.Zero(t, legacyCalls.Load(), "the attempt path must not call ExecuteTask")
}

func TestProcessTaskCreatedEvents_AttemptPathNacksAnUnsettledDeliveryAfterTheDelay(t *testing.T) {
	t.Parallel()

	const delay = 50 * time.Millisecond
	ec := NewEventConsumer(&fakeDownloadService{}, nil, log.NewNopLogger(),
		WithAttemptRunner(&fakeAttemptRunner{runFn: func(context.Context, download.TaskRequest) error {
			return &apperrors.Error{Code: apperrors.ErrCodeInternal, Message: "failed to claim task"}
		}}),
		WithRedeliveryDelay(delay),
	)

	ch := make(chan *message.Message, 1)
	msg := createdMessage(t, 7)
	ch <- msg
	close(ch)

	start := time.Now()
	ec.processTaskCreatedEvents(t.Context(), ch)

	assert.False(t, waitAckOrNack(t, msg), "an unsettled delivery must not be acknowledged")
	assert.GreaterOrEqual(t, time.Since(start), delay, "the negative acknowledgement must be delayed")
}

func TestProcessTaskCreatedEvents_AttemptPathRunsDeliveriesConcurrently(t *testing.T) {
	t.Parallel()

	started := make(chan uint64, 2)
	release := make(chan struct{})
	ec := NewEventConsumer(&fakeDownloadService{}, nil, log.NewNopLogger(),
		WithAttemptRunner(&fakeAttemptRunner{runFn: func(_ context.Context, req download.TaskRequest) error {
			started <- req.TaskID
			<-release
			return nil
		}}),
	)

	ch := make(chan *message.Message, 2)
	first, second := createdMessage(t, 1), createdMessage(t, 2)
	ch <- first
	ch <- second
	close(ch)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ec.processTaskCreatedEvents(t.Context(), ch)
	}()

	// Both attempts run although neither has settled: one slow download must
	// not hold back the next dispatch.
	seen := map[uint64]bool{}
	for range 2 {
		select {
		case id := <-started:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatal("the second attempt did not start while the first was running")
		}
	}
	assert.Equal(t, map[uint64]bool{1: true, 2: true}, seen)

	close(release)
	assert.True(t, waitAckOrNack(t, first))
	assert.True(t, waitAckOrNack(t, second))
	<-done
}

func TestStart_WaitsForRunningAttemptsOnShutdown(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	var settled atomic.Bool
	runner := &fakeAttemptRunner{runFn: func(ctx context.Context, _ download.TaskRequest) error {
		close(started)
		<-ctx.Done()
		// Recording the outcome (a release) takes a moment after the cancel.
		time.Sleep(50 * time.Millisecond)
		settled.Store(true)
		return nil
	}}
	pub, sub := inmem.NewPublisherAndSubscriber(log.NewNopLogger())
	ec := NewEventConsumer(&fakeDownloadService{}, sub, log.NewNopLogger(), WithAttemptRunner(runner))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- ec.Start(ctx) }()

	// The subscription is set up asynchronously: publish once it exists. The
	// bus delivers a copy, so the same message can be offered again.
	dispatch := createdMessage(t, 5)
	require.Eventually(t, func() bool {
		return pub.PublishRequiringSubscriber("task.created", dispatch) == nil
	}, 2*time.Second, 5*time.Millisecond)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the attempt never started")
	}

	cancel()
	select {
	case err := <-stopped:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after ctx was cancelled")
	}
	assert.True(t, settled.Load(), "Start returned before the running attempt settled")
}

// fakeAdmittingRunner is a scripted download.AdmittingRunner with a fixed
// number of slots.
type fakeAdmittingRunner struct {
	slots    chan struct{}
	admitted atomic.Int32
	runFn    func(ctx context.Context, req download.TaskRequest) error
}

func (f *fakeAdmittingRunner) RunAttempt(ctx context.Context, req download.TaskRequest) error {
	return f.runFn(ctx, req)
}

func (f *fakeAdmittingRunner) Admit(ctx context.Context) error {
	select {
	case f.slots <- struct{}{}:
		f.admitted.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeAdmittingRunner) RunAdmittedAttempt(ctx context.Context, req download.TaskRequest) error {
	defer func() { <-f.slots }()
	return f.runFn(ctx, req)
}

func TestProcessTaskCreatedEvents_AttemptPathAdmitsBeforeSpawning(t *testing.T) {
	t.Parallel()

	started := make(chan uint64, 3)
	release := make(chan struct{})
	runner := &fakeAdmittingRunner{
		slots: make(chan struct{}, 1),
		runFn: func(_ context.Context, req download.TaskRequest) error {
			started <- req.TaskID
			<-release
			return nil
		},
	}
	ec := NewEventConsumer(&fakeDownloadService{}, nil, log.NewNopLogger(), WithAttemptRunner(runner))

	ch := make(chan *message.Message, 3)
	msgs := []*message.Message{createdMessage(t, 1), createdMessage(t, 2), createdMessage(t, 3)}
	for _, m := range msgs {
		ch <- m
	}
	close(ch)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ec.processTaskCreatedEvents(t.Context(), ch)
	}()

	require.Equal(t, uint64(1), <-started)
	select {
	case id := <-started:
		t.Fatalf("task %d ran beyond the single slot", id)
	case <-time.After(100 * time.Millisecond):
	}
	assert.EqualValues(t, 1, runner.admitted.Load(), "no further delivery may be admitted while the slot is held")
	assert.Len(t, ch, 1, "the backlog stays in the subscription, not in goroutines")

	close(release)
	for _, m := range msgs {
		assert.True(t, waitAckOrNack(t, m))
	}
	<-done
	assert.EqualValues(t, 3, runner.admitted.Load())
}
