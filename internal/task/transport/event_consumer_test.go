package tasktransport

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/internal/events"
	"github.com/yuisofull/goload/internal/storage"
	"github.com/yuisofull/goload/internal/task"
	"github.com/yuisofull/goload/pkg/message"
	"github.com/yuisofull/goload/pkg/message/inmem"
)

// recordingTaskService records the unfenced task writes the legacy lifecycle
// handlers perform. Every other task.Service method is left nil and panics
// when it is called.
type recordingTaskService struct {
	task.Service

	mu    sync.Mutex
	calls []string
}

func (s *recordingTaskService) record(call string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
	return nil
}

func (s *recordingTaskService) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func (s *recordingTaskService) UpdateTaskProgress(context.Context, uint64, task.DownloadProgress) error {
	return s.record("UpdateTaskProgress")
}

func (s *recordingTaskService) CompleteTask(context.Context, uint64) error {
	return s.record("CompleteTask")
}

func (s *recordingTaskService) UpdateFileName(context.Context, uint64, string) error {
	return s.record("UpdateFileName")
}

func (s *recordingTaskService) UpdateTaskChecksum(context.Context, uint64, *task.ChecksumInfo) error {
	return s.record("UpdateTaskChecksum")
}

func (s *recordingTaskService) UpdateStorageInfo(context.Context, uint64, storage.Type, string) error {
	return s.record("UpdateStorageInfo")
}

func (s *recordingTaskService) UpdateTaskError(context.Context, uint64, error) error {
	return s.record("UpdateTaskError")
}

// lifecycleEvents is one event per lifecycle topic, all for task 42, as a
// download worker of the legacy path (or a stale attempt) publishes them.
func lifecycleEvents(t *testing.T) map[string]*message.Message {
	t.Helper()

	marshal := func(event any) *message.Message {
		payload, err := json.Marshal(event)
		require.NoError(t, err)
		return message.NewMessage("event", payload)
	}
	return map[string]*message.Message{
		"task.progress.updated": marshal(events.TaskProgressUpdatedEvent{
			TaskID:          42,
			Progress:        10,
			DownloadedBytes: 100,
			TotalBytes:      1000,
		}),
		"task.completed": marshal(events.TaskCompletedEvent{
			TaskID:      42,
			FileName:    "file.bin",
			FileSize:    1000,
			Checksum:    &events.ChecksumInfo{ChecksumType: "md5", ChecksumValue: "abc123"},
			StorageType: "local",
			StorageKey:  "42/1/file-0011223344556677.bin",
		}),
		"task.failed": marshal(events.TaskFailedEvent{TaskID: 42, Error: "stale failure"}),
	}
}

// deliver runs every lifecycle event through its handler loop and returns,
// per topic, whether the event was acknowledged.
func deliver(t *testing.T, ec *EventConsumer) map[string]bool {
	t.Helper()

	handlers := map[string]func(context.Context, <-chan *message.Message){
		"task.progress.updated": ec.handleProgressUpdates,
		"task.completed":        ec.handleCompletions,
		"task.failed":           ec.handleFailures,
	}
	acked := map[string]bool{}
	for topic, msg := range lifecycleEvents(t) {
		ch := make(chan *message.Message, 1)
		ch <- msg
		close(ch)
		handlers[topic](t.Context(), ch)
		select {
		case <-msg.Acked():
			acked[topic] = true
		default:
			acked[topic] = false
		}
	}
	return acked
}

func allAcked() map[string]bool {
	return map[string]bool{"task.progress.updated": true, "task.completed": true, "task.failed": true}
}

func TestEventConsumer_LifecycleEventsAreNotificationsOnly(t *testing.T) {
	svc := &recordingTaskService{}
	ec := NewEventConsumer(svc, nil, func(_ context.Context, err error) {
		t.Errorf("unexpected consumer error: %v", err)
	})

	acked := deliver(t, ec)

	require.Equal(t, allAcked(), acked, "every notification is acknowledged")
	require.Empty(t, svc.snapshot(),
		"a lifecycle event carries no generation and must never write the task row: "+
			"a stale failure or progress event would overwrite the winner")
}

func TestEventConsumer_LegacyStateWritesApplyEvents(t *testing.T) {
	svc := &recordingTaskService{}
	ec := NewEventConsumer(svc, nil, func(_ context.Context, err error) {
		t.Errorf("unexpected consumer error: %v", err)
	}, WithLegacyStateWrites())

	acked := deliver(t, ec)

	require.Equal(t, allAcked(), acked)
	require.ElementsMatch(t, []string{
		// task.progress.updated
		"UpdateTaskProgress",
		// task.completed
		"CompleteTask", "UpdateFileName", "UpdateTaskChecksum", "UpdateStorageInfo", "UpdateTaskProgress",
		// task.failed
		"UpdateTaskError",
	}, svc.snapshot())
}

func TestEventConsumer_KeepsLifecycleTopicsSubscribed(t *testing.T) {
	pub, sub := inmem.NewPublisherAndSubscriber(log.NewNopLogger())
	ec := NewEventConsumer(&recordingTaskService{}, sub, func(context.Context, error) {})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- ec.Start(ctx) }()

	// The outbox relay publishes lifecycle rows with PublishRequiringSubscriber
	// and keeps a row unsent while nobody is subscribed.
	for _, topic := range []string{"task.progress.updated", "task.completed", "task.failed"} {
		require.Eventually(t, func() bool {
			probe := message.NewMessage("probe", []byte(`{"task_id":1}`))
			return pub.PublishRequiringSubscriber(topic, probe) == nil
		}, 2*time.Second, 5*time.Millisecond, topic)
	}

	cancel()
	require.ErrorIs(t, <-stopped, context.Canceled)
}
