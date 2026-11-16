package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/internal/events"
	task "github.com/yuisofull/goload/internal/task"
	"github.com/yuisofull/goload/pkg/message/inmem"
)

// failAfterInsertOutbox writes the row and then reports failure, so the test
// proves the transaction (not a skipped write) removes it.
type failAfterInsertOutbox struct {
	task.OutboxRepository
	err error
}

func (o failAfterInsertOutbox) InsertUnsent(ctx context.Context, event task.OutboxEvent) error {
	if err := o.OutboxRepository.InsertUnsent(ctx, event); err != nil {
		return err
	}
	return o.err
}

func TestCreateTask_CommitsOutboxRowThenRelayPublishes(t *testing.T) {
	pool := newTestPool(t)
	repo := NewTaskRepo(pool)
	outbox := NewOutboxRepo(pool)
	msgPub, msgSub := inmem.NewPublisherAndSubscriber(log.NewNopLogger())
	taskPub := task.NewEventPublisher(msgPub)
	svc := task.NewService(repo, *taskPub, NewTxManager(pool), outbox)
	ctx := context.Background()

	ch, err := msgSub.Subscribe(t.Context(), "task.created")
	require.NoError(t, err)

	created, err := svc.CreateTask(ctx, &task.CreateTaskParam{
		OfAccountID: 7,
		FileName:    "file.bin",
		SourceURL:   "https://example.test/file.bin",
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	// Committed but not published yet: the event waits in the outbox.
	unsent, err := outbox.FetchUnsent(ctx, task.OutboxCursor{}, 10)
	require.NoError(t, err)
	require.Len(t, unsent, 1)
	require.Equal(t, created.ID, unsent[0].AggregateID)
	select {
	case <-ch:
		t.Fatal("CreateTask must not publish directly")
	case <-time.After(50 * time.Millisecond):
	}

	relay := task.NewOutboxRelay(outbox, taskPub)
	sent, err := relay.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, sent)

	select {
	case msg := <-ch:
		require.NotNil(t, msg)
		require.Equal(t, unsent[0].ID, msg.UUID)
		require.Equal(t, "TaskCreated", msg.Metadata.Get("eventType"))
		var event events.TaskCreatedEvent
		require.NoError(t, json.Unmarshal(msg.Payload, &event))
		require.Equal(t, created.ID, event.TaskID)
		require.Equal(t, uint64(7), event.OfAccountID)
		require.Equal(t, "https://example.test/file.bin", event.SourceURL)
		msg.Ack()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for task.created message")
	}

	unsent, err = outbox.FetchUnsent(ctx, task.OutboxCursor{}, 10)
	require.NoError(t, err)
	require.Empty(t, unsent)
}

func TestCreateTask_OutboxFailureRollsBackTaskRow(t *testing.T) {
	pool := newTestPool(t)
	repo := NewTaskRepo(pool)
	outbox := NewOutboxRepo(pool)
	boom := errors.New("outbox insert failed")
	svc := task.NewService(
		repo, task.Publisher{}, NewTxManager(pool),
		failAfterInsertOutbox{OutboxRepository: outbox, err: boom},
	)
	ctx := context.Background()

	_, err := svc.CreateTask(ctx, &task.CreateTaskParam{
		OfAccountID: 7,
		SourceURL:   "https://example.test/file.bin",
	})
	require.ErrorIs(t, err, boom)

	count, err := repo.GetTaskCountOfAccount(ctx, 7)
	require.NoError(t, err)
	require.Zero(t, count, "the task row must roll back with the outbox row")

	unsent, err := outbox.FetchUnsent(ctx, task.OutboxCursor{}, 10)
	require.NoError(t, err)
	require.Empty(t, unsent)
}

func TestRetryTask_CommitsPendingStatusAndOutboxRow(t *testing.T) {
	pool := newTestPool(t)
	repo := NewTaskRepo(pool)
	outbox := NewOutboxRepo(pool)
	svc := task.NewService(repo, task.Publisher{}, NewTxManager(pool), outbox)
	ctx := context.Background()

	failed := pendingTask(7)
	failed.Status = task.StatusFailed
	created, err := repo.Create(ctx, failed)
	require.NoError(t, err)

	require.NoError(t, svc.RetryTask(ctx, created.ID))

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, task.StatusPending, got.Status)

	unsent, err := outbox.FetchUnsent(ctx, task.OutboxCursor{}, 10)
	require.NoError(t, err)
	require.Len(t, unsent, 1)
	require.Equal(t, created.ID, unsent[0].AggregateID)
	require.Equal(t, task.OutboxEventTaskCreated, unsent[0].EventType)
}

func TestRetryTask_OutboxFailureKeepsTaskFailed(t *testing.T) {
	pool := newTestPool(t)
	repo := NewTaskRepo(pool)
	outbox := NewOutboxRepo(pool)
	boom := errors.New("outbox insert failed")
	svc := task.NewService(
		repo, task.Publisher{}, NewTxManager(pool),
		failAfterInsertOutbox{OutboxRepository: outbox, err: boom},
	)
	ctx := context.Background()

	failed := pendingTask(7)
	failed.Status = task.StatusFailed
	created, err := repo.Create(ctx, failed)
	require.NoError(t, err)

	require.ErrorIs(t, svc.RetryTask(ctx, created.ID), boom)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, task.StatusFailed, got.Status, "the status update must roll back with the outbox row")

	unsent, err := outbox.FetchUnsent(ctx, task.OutboxCursor{}, 10)
	require.NoError(t, err)
	require.Empty(t, unsent)
}
