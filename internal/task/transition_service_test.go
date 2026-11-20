package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apperrors "github.com/yuisofull/goload/internal/errors"
)

func ownedAttemptFixture(status TaskStatus) AttemptState {
	lease := time.Now().Add(time.Minute)
	return AttemptState{TaskID: 55, Status: status, OwnerID: "worker-a", Generation: 1, LeaseExpiresAt: &lease}
}

func TestCancelTask_ClearsOwnershipAndPublishesCancelled(t *testing.T) {
	attempts := newMemAttempts(ownedAttemptFixture(StatusStoring))
	transport := &fakeMessagePublisher{}
	svc := NewService(&fakeRepo{}, *NewEventPublisher(transport), fakeTxManager{}, &memOutbox{}, attempts)

	require.NoError(t, svc.CancelTask(context.Background(), 55))

	st, err := attempts.GetAttemptState(context.Background(), 55)
	require.NoError(t, err)
	require.Equal(t, StatusCancelled, st.Status)
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Equal(t, uint64(1), st.Generation)
	require.Equal(t, "task.cancelled", transport.topic)
	require.Len(t, transport.msgs, 1)
}

func TestPauseTask_RetiresAttemptAndPublishesPaused(t *testing.T) {
	attempts := newMemAttempts(ownedAttemptFixture(StatusDownloading))
	transport := &fakeMessagePublisher{}
	svc := NewService(&fakeRepo{}, *NewEventPublisher(transport), fakeTxManager{}, &memOutbox{}, attempts)

	require.NoError(t, svc.PauseTask(context.Background(), 55))

	st, err := attempts.GetAttemptState(context.Background(), 55)
	require.NoError(t, err)
	require.Equal(t, StatusPaused, st.Status)
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Equal(t, "task.paused", transport.topic)
	require.Len(t, transport.msgs, 1)
}

func TestResumeTask_RequeuesThroughOutboxAndDoesNotPublish(t *testing.T) {
	repo := &stubGetRepo{task: failedTaskFixture()} // task 55; only its fields feed the payload
	attempts := newMemAttempts(AttemptState{TaskID: 55, Status: StatusPaused, Generation: 1})
	outbox := &memOutbox{}
	transport := &fakeMessagePublisher{}
	svc := NewService(repo, *NewEventPublisher(transport), fakeTxManager{}, outbox, attempts)

	require.NoError(t, svc.ResumeTask(context.Background(), 55))

	st, err := attempts.GetAttemptState(context.Background(), 55)
	require.NoError(t, err)
	require.Equal(t, StatusPending, st.Status)
	require.Empty(t, transport.msgs, "ResumeTask re-dispatches through the outbox, not task.resumed")
	queued := outbox.snapshot()
	require.Len(t, queued, 1)
	require.Equal(t, uint64(55), queued[0].AggregateID)
	require.Equal(t, OutboxDestinationDispatch, queued[0].Destination)
	require.Equal(t, OutboxEventTaskCreated, queued[0].EventType)
}

func TestUserTransitions_RejectedTransitionWritesNothing(t *testing.T) {
	attempts := newMemAttempts(AttemptState{TaskID: 55, Status: StatusCompleted, Generation: 2})
	outbox := &memOutbox{}
	transport := &fakeMessagePublisher{}
	svc := NewService(&fakeRepo{}, *NewEventPublisher(transport), fakeTxManager{}, outbox, attempts)

	for name, call := range map[string]func(context.Context, uint64) error{
		"pause":  svc.PauseTask,
		"resume": svc.ResumeTask,
		"cancel": svc.CancelTask,
		"retry":  svc.RetryTask,
	} {
		err := call(context.Background(), 55)
		require.True(t, apperrors.IsError(err, apperrors.ErrCodeInvalidInput), name)
	}

	st, err := attempts.GetAttemptState(context.Background(), 55)
	require.NoError(t, err)
	require.Equal(t, StatusCompleted, st.Status)
	require.Empty(t, transport.msgs)
	require.Empty(t, outbox.snapshot())
}

func TestUserTransitions_UnknownTaskIsNotFound(t *testing.T) {
	svc := NewService(&fakeRepo{}, Publisher{}, fakeTxManager{}, &memOutbox{}, newMemAttempts())

	for name, call := range map[string]func(context.Context, uint64) error{
		"pause":  svc.PauseTask,
		"resume": svc.ResumeTask,
		"cancel": svc.CancelTask,
		"retry":  svc.RetryTask,
	} {
		err := call(context.Background(), 404)
		require.True(t, apperrors.IsError(err, apperrors.ErrCodeNotFound), name)
	}
}
