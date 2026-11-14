package task

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/internal/storage"
)

func TestOutboxConstants(t *testing.T) {
	require.Equal(t, "dispatch", OutboxDestinationDispatch)
	require.Equal(t, "lifecycle", OutboxDestinationLifecycle)
	require.Equal(t, "TaskCreated", OutboxEventTaskCreated)
	require.Equal(t, "TaskCompleted", OutboxEventTaskCompleted)
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

func TestNewTaskCompletedOutboxEvent_Payload(t *testing.T) {
	before := time.Now().UTC()
	completedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	event, err := NewTaskCompletedOutboxEvent(42, AttemptResult{
		FileName:    "resolved.bin",
		FileSize:    1000,
		ContentType: "application/octet-stream",
		Checksum:    &ChecksumInfo{ChecksumType: "md5", ChecksumValue: "abc123"},
		StorageType: storage.TypeLocal,
		StorageKey:  "42/1/resolved-0011223344556677.bin",
	}, completedAt)
	require.NoError(t, err)

	// The exact events.TaskCompletedEvent JSON used on task.completed today.
	require.Equal(t,
		`{"task_id":42,"file_name":"resolved.bin","file_size":1000,`+
			`"content_type":"application/octet-stream",`+
			`"checksum":{"checksum_type":"md5","checksum_value":"abc123"},`+
			`"storage_type":"local","storage_key":"42/1/resolved-0011223344556677.bin",`+
			`"completed_at":"2026-01-02T03:04:05Z"}`,
		string(event.Payload),
	)
	require.Equal(t, uint64(42), event.AggregateID)
	require.Equal(t, OutboxDestinationLifecycle, event.Destination)
	require.Equal(t, OutboxEventTaskCompleted, event.EventType)
	require.Nil(t, event.SentAt)
	require.False(t, event.CreatedAt.Before(before), "created_at must be set when the event is built")

	_, err = uuid.Parse(event.ID)
	require.NoError(t, err, "event id must be a UUID")
}

func TestNewTaskCompletedOutboxEvent_OmitsMissingChecksum(t *testing.T) {
	event, err := NewTaskCompletedOutboxEvent(7, AttemptResult{
		StorageType: storage.TypeMinio,
		StorageKey:  "7/2/file-0011223344556677.bin",
	}, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	require.NoError(t, err)

	require.Equal(t,
		`{"task_id":7,"file_name":"","file_size":0,"content_type":"",`+
			`"storage_type":"minio","storage_key":"7/2/file-0011223344556677.bin",`+
			`"completed_at":"2026-01-02T03:04:05Z"}`,
		string(event.Payload),
	)
}
