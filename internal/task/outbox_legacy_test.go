package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests pin the wire format of the legacy direct publish. The whole
// file is deleted together with Publisher.PublishTaskCreated at the end of
// the outbox work.

func TestPublishTaskCreated_LegacyWireFormat(t *testing.T) {
	transport := &fakeMessagePublisher{}
	pub := NewEventPublisher(transport)

	require.NoError(t, pub.PublishTaskCreated(context.Background(), sampleOutboxTask()))

	require.Equal(t, "task.created", transport.topic)
	require.Len(t, transport.msgs, 1)
	require.Equal(t, taskCreatedGoldenPayload, string(transport.msgs[0].Payload))
	require.Equal(t, "TaskCreated", transport.msgs[0].Metadata.Get("eventType"))
	require.Equal(t, "42", transport.msgs[0].Metadata.Get("taskID"))
}

func TestPublishTaskCreated_LegacyWireFormat_FreshTask(t *testing.T) {
	transport := &fakeMessagePublisher{}
	pub := NewEventPublisher(transport)

	require.NoError(t, pub.PublishTaskCreated(context.Background(), freshOutboxTask()))

	require.Len(t, transport.msgs, 1)
	require.Equal(t, freshTaskCreatedGoldenPayload, string(transport.msgs[0].Payload))
}

func TestPublishOutboxEvent_MatchesLegacyPublishTaskCreated(t *testing.T) {
	legacy := &recordingPublisher{}
	require.NoError(t, NewEventPublisher(legacy).PublishTaskCreated(context.Background(), sampleOutboxTask()))

	event, err := NewTaskCreatedOutboxEvent(sampleOutboxTask())
	require.NoError(t, err)
	viaOutbox := &recordingPublisher{}
	require.NoError(t, NewEventPublisher(viaOutbox).PublishOutboxEvent(context.Background(), event))

	want, got := legacy.snapshot(), viaOutbox.snapshot()
	require.Len(t, want, 1)
	require.Len(t, got, 1)
	require.Equal(t, want[0].topic, got[0].topic)
	require.Equal(t, want[0].msg.Metadata, got[0].msg.Metadata)
	require.Equal(t, string(want[0].msg.Payload), string(got[0].msg.Payload))
	require.Equal(t, event.ID, got[0].msg.UUID, "the outbox id is reused as the message UUID")
}
