package task

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/yuisofull/goload/pkg/message"
	"github.com/yuisofull/goload/pkg/message/inmem"
)

var errPublishFailed = errors.New("publish failed")

type publishedMessage struct {
	topic string
	msg   *message.Message
}

// recordingPublisher is a message.Publisher that records every publish. It
// can fail the first attempts for a message UUID and run a hook per publish.
type recordingPublisher struct {
	mu        sync.Mutex
	published []publishedMessage
	attempts  map[string]int // publish attempts per message UUID
	failFirst map[string]int // fail this many first attempts per message UUID
	onPublish func()
}

func (p *recordingPublisher) Publish(topic string, messages ...*message.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.attempts == nil {
		p.attempts = map[string]int{}
	}
	for _, msg := range messages {
		p.attempts[msg.UUID]++
		if p.attempts[msg.UUID] <= p.failFirst[msg.UUID] {
			return errPublishFailed
		}
		p.published = append(p.published, publishedMessage{topic: topic, msg: msg})
		if p.onPublish != nil {
			p.onPublish()
		}
	}
	return nil
}

func (p *recordingPublisher) Close() error { return nil }

func (p *recordingPublisher) snapshot() []publishedMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]publishedMessage(nil), p.published...)
}

func (p *recordingPublisher) uuids() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.published))
	for _, pm := range p.published {
		out = append(out, pm.msg.UUID)
	}
	return out
}

func outboxEventFixture(id string, taskID uint64) OutboxEvent {
	return OutboxEvent{
		ID:          id,
		AggregateID: taskID,
		Destination: OutboxDestinationDispatch,
		EventType:   OutboxEventTaskCreated,
		Payload:     []byte(`{"task_id":` + strconv.FormatUint(taskID, 10) + `}`),
		CreatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestPublishOutboxEvent_PublishesOnTaskCreatedTopic(t *testing.T) {
	transport := &recordingPublisher{}
	pub := NewEventPublisher(transport)

	require.NoError(t, pub.PublishOutboxEvent(context.Background(), outboxEventFixture("evt-1", 42)))

	got := transport.snapshot()
	require.Len(t, got, 1)
	require.Equal(t, "task.created", got[0].topic)
	require.Equal(t, "evt-1", got[0].msg.UUID)
	require.Equal(t, `{"task_id":42}`, string(got[0].msg.Payload))
	require.Equal(t, message.Metadata{"eventType": "TaskCreated", "taskID": "42"}, got[0].msg.Metadata)
}

func TestPublishOutboxEvent_UnknownEventTypeIsNotPublished(t *testing.T) {
	transport := &recordingPublisher{}
	pub := NewEventPublisher(transport)
	event := outboxEventFixture("evt-1", 42)
	event.EventType = "TaskTeleported"

	err := pub.PublishOutboxEvent(context.Background(), event)

	require.Error(t, err)
	require.Contains(t, err.Error(), "TaskTeleported")
	require.Empty(t, transport.snapshot())
}

func TestPublishOutboxEvent_NoTransportConfigured(t *testing.T) {
	// cmd/task wraps a nil message.Publisher when KAFKA_BROKERS is empty.
	pub := NewEventPublisher(nil)
	require.Error(t, pub.PublishOutboxEvent(context.Background(), outboxEventFixture("evt-1", 42)))

	var nilPub *Publisher
	require.Error(t, nilPub.PublishOutboxEvent(context.Background(), outboxEventFixture("evt-1", 42)))
}

func TestPublishOutboxEvent_InmemRequiresSubscriber(t *testing.T) {
	msgPub, msgSub := inmem.NewPublisherAndSubscriber(log.NewNopLogger())
	pub := NewEventPublisher(msgPub)
	event := outboxEventFixture("evt-1", 42)

	// Nobody listens yet: the in-memory bus would drop the message.
	err := pub.PublishOutboxEvent(context.Background(), event)
	require.ErrorIs(t, err, ErrOutboxNoSubscribers)

	ch, err := msgSub.Subscribe(t.Context(), "task.created")
	require.NoError(t, err)

	require.NoError(t, pub.PublishOutboxEvent(context.Background(), event))

	select {
	case msg := <-ch:
		require.NotNil(t, msg)
		require.Equal(t, "evt-1", msg.UUID)
		require.Equal(t, `{"task_id":42}`, string(msg.Payload))
		require.Equal(t, "TaskCreated", msg.Metadata.Get("eventType"))
		require.Equal(t, "42", msg.Metadata.Get("taskID"))
		msg.Ack()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for task.created message")
	}
}
