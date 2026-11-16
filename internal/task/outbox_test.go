package task

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOutboxConstants(t *testing.T) {
	require.Equal(t, "dispatch", OutboxDestinationDispatch)
	require.Equal(t, "lifecycle", OutboxDestinationLifecycle)
	require.Equal(t, "TaskCreated", OutboxEventTaskCreated)
}

func TestNewTaskCreatedOutboxEvent_ReproducesLegacyPayload(t *testing.T) {
	before := time.Now().UTC()

	event, err := NewTaskCreatedOutboxEvent(sampleOutboxTask())
	require.NoError(t, err)

	require.Equal(t, taskCreatedGoldenPayload, string(event.Payload))
	require.Equal(t, uint64(42), event.AggregateID)
	require.Equal(t, OutboxDestinationDispatch, event.Destination)
	require.Equal(t, OutboxEventTaskCreated, event.EventType)
	require.Nil(t, event.SentAt)
	require.False(t, event.CreatedAt.Before(before), "created_at must be set when the event is built")
	require.Equal(t, time.UTC, event.CreatedAt.Location())

	_, err = uuid.Parse(event.ID)
	require.NoError(t, err, "event id must be a UUID")
}

func TestNewTaskCreatedOutboxEvent_FreshTaskOmitsOptionalSections(t *testing.T) {
	event, err := NewTaskCreatedOutboxEvent(freshOutboxTask())
	require.NoError(t, err)

	require.Equal(t, freshTaskCreatedGoldenPayload, string(event.Payload))
	require.Equal(t, uint64(1), event.AggregateID)
}

func TestNewTaskCreatedOutboxEvent_UniqueIDs(t *testing.T) {
	first, err := NewTaskCreatedOutboxEvent(sampleOutboxTask())
	require.NoError(t, err)
	second, err := NewTaskCreatedOutboxEvent(sampleOutboxTask())
	require.NoError(t, err)

	require.NotEqual(t, first.ID, second.ID)
}
