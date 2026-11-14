package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	apperrors "github.com/yuisofull/goload/internal/errors"
	"github.com/yuisofull/goload/internal/events"
)

// stubGetRepo is fakeRepo with a configurable GetByID result.
type stubGetRepo struct {
	fakeRepo
	task *Task
}

func (r *stubGetRepo) GetByID(context.Context, uint64) (*Task, error) { return r.task, nil }

func failedTaskFixture() *Task {
	return &Task{
		ID:          55,
		OfAccountID: 7,
		FileName:    "file.bin",
		SourceURL:   "https://example.test/file.bin",
		SourceType:  SourceHTTPS,
		Status:      StatusFailed,
	}
}

func TestCreateTask_WritesOutboxRowAndDoesNotPublish(t *testing.T) {
	repo := &fakeRepo{}
	outbox := &memOutbox{}
	transport := &fakeMessagePublisher{}
	svc := NewService(repo, *NewEventPublisher(transport), fakeTxManager{}, outbox)

	created, err := svc.CreateTask(context.Background(), &CreateTaskParam{
		OfAccountID: 7,
		FileName:    "file.bin",
		SourceURL:   "https://example.test/file.bin",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(123), created.ID)

	require.Empty(t, transport.msgs, "CreateTask must not publish directly any more")

	queued := outbox.snapshot()
	require.Len(t, queued, 1)
	require.NotEmpty(t, queued[0].ID)
	require.Equal(t, uint64(123), queued[0].AggregateID)
	require.Equal(t, OutboxDestinationDispatch, queued[0].Destination)
	require.Equal(t, OutboxEventTaskCreated, queued[0].EventType)
	require.Nil(t, queued[0].SentAt)

	var payload events.TaskCreatedEvent
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	require.Equal(t, uint64(123), payload.TaskID)
	require.Equal(t, uint64(7), payload.OfAccountID)
	require.Equal(t, "file.bin", payload.FileName)
	require.Equal(t, "https://example.test/file.bin", payload.SourceURL)
	require.Equal(t, "HTTPS", payload.SourceType)
	require.NotNil(t, payload.DownloadOptions)
	require.Equal(t, 16, payload.DownloadOptions.Concurrency)
	require.Equal(t, 3, payload.DownloadOptions.MaxRetries)
}

func TestCreateTask_OutboxInsertFailureFailsCreate(t *testing.T) {
	insertErr := errors.New("outbox insert failed")
	outbox := &memOutbox{insertErr: insertErr}
	transport := &fakeMessagePublisher{}
	svc := NewService(&fakeRepo{}, *NewEventPublisher(transport), fakeTxManager{}, outbox)

	created, err := svc.CreateTask(context.Background(), &CreateTaskParam{
		OfAccountID: 7,
		SourceURL:   "https://example.test/file.bin",
	})

	require.Nil(t, created)
	require.ErrorIs(t, err, insertErr)
	require.True(t, apperrors.IsError(err, apperrors.ErrCodeInternal))
	require.Empty(t, transport.msgs)
	require.Empty(t, outbox.snapshot())
}

func TestRetryTask_WritesOutboxRowAndDoesNotPublish(t *testing.T) {
	repo := &stubGetRepo{task: failedTaskFixture()}
	outbox := &memOutbox{}
	transport := &fakeMessagePublisher{}
	svc := NewService(repo, *NewEventPublisher(transport), fakeTxManager{}, outbox)

	require.NoError(t, svc.RetryTask(context.Background(), 55))

	require.NotNil(t, repo.updated)
	require.Equal(t, StatusPending, repo.updated.Status)
	require.Empty(t, transport.msgs, "RetryTask must not publish directly any more")

	queued := outbox.snapshot()
	require.Len(t, queued, 1)
	require.Equal(t, uint64(55), queued[0].AggregateID)
	require.Equal(t, OutboxDestinationDispatch, queued[0].Destination)
	require.Equal(t, OutboxEventTaskCreated, queued[0].EventType)

	var payload events.TaskCreatedEvent
	require.NoError(t, json.Unmarshal(queued[0].Payload, &payload))
	require.Equal(t, uint64(55), payload.TaskID)
	require.Equal(t, "https://example.test/file.bin", payload.SourceURL)
}

func TestRetryTask_OutboxInsertFailureFailsRetry(t *testing.T) {
	insertErr := errors.New("outbox insert failed")
	repo := &stubGetRepo{task: failedTaskFixture()}
	outbox := &memOutbox{insertErr: insertErr}
	transport := &fakeMessagePublisher{}
	svc := NewService(repo, *NewEventPublisher(transport), fakeTxManager{}, outbox)

	err := svc.RetryTask(context.Background(), 55)

	require.ErrorIs(t, err, insertErr)
	require.True(t, apperrors.IsError(err, apperrors.ErrCodeInternal))
	require.Empty(t, transport.msgs)
}
