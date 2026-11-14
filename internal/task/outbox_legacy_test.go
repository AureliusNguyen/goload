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
