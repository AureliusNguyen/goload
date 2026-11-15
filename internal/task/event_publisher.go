package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/yuisofull/goload/internal/events"
	"github.com/yuisofull/goload/pkg/message"
)

// Publisher publishes task-related events
type Publisher struct {
	publisher message.Publisher
	outbox    OutboxTransport
}

// EventPublisherOption configures a Publisher.
type EventPublisherOption func(*Publisher)

// WithOutboxTransport sets the transport used to deliver outbox events, making
// the "publish only when a subscriber exists" choice explicit at wiring time.
// When it is not set, outbox events use a RetainingTransport over the same
// publisher, which delivers unconditionally (the Kafka semantics).
func WithOutboxTransport(t OutboxTransport) EventPublisherOption {
	return func(ep *Publisher) {
		if t != nil {
			ep.outbox = t
		}
	}
}

// NewEventPublisher creates a new event publisher for task service
func NewEventPublisher(publisher message.Publisher, opts ...EventPublisherOption) *Publisher {
	ep := &Publisher{publisher: publisher}
	// Default outbox delivery retains without a subscriber (Kafka semantics).
	// A nil publisher leaves no transport, so PublishOutboxEvent reports it
	// rather than dereferencing nil.
	if publisher != nil {
		ep.outbox = NewRetainingTransport(publisher)
	}
	for _, o := range opts {
		o(ep)
	}
	return ep
}

// PublishTaskStatusUpdated publishes a task status update event
func (ep *Publisher) PublishTaskStatusUpdated(ctx context.Context, taskID uint64, status TaskStatus) error {
	event := events.TaskStatusUpdatedEvent{
		TaskID:    taskID,
		Status:    events.TaskStatusValue(string(status)),
		UpdatedAt: time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &message.Message{
		UUID:    generateUUID(),
		Payload: payload,
		Metadata: message.Metadata{
			"eventType": "TaskStatusUpdated",
			"taskID":    formatTaskID(taskID),
		},
	}

	return ep.publisher.Publish("task.status.updated", msg)
}

// PublishTaskPaused publishes a task paused event
func (ep *Publisher) PublishTaskPaused(ctx context.Context, taskID uint64) error {
	event := events.TaskPausedEvent{
		TaskID:   taskID,
		PausedAt: time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &message.Message{
		UUID:    generateUUID(),
		Payload: payload,
		Metadata: message.Metadata{
			"eventType": "TaskPaused",
			"taskID":    formatTaskID(taskID),
		},
	}

	return ep.publisher.Publish("task.paused", msg)
}

// PublishTaskResumed publishes a task resumed event
func (ep *Publisher) PublishTaskResumed(ctx context.Context, taskID uint64) error {
	event := events.TaskResumedEvent{
		TaskID:    taskID,
		ResumedAt: time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &message.Message{
		UUID:    generateUUID(),
		Payload: payload,
		Metadata: message.Metadata{
			"eventType": "TaskResumed",
			"taskID":    formatTaskID(taskID),
		},
	}

	return ep.publisher.Publish("task.resumed", msg)
}

// PublishTaskCancelled publishes a task cancelled event
func (ep *Publisher) PublishTaskCancelled(ctx context.Context, taskID uint64) error {
	event := events.TaskCancelledEvent{
		TaskID:      taskID,
		CancelledAt: time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &message.Message{
		UUID:    generateUUID(),
		Payload: payload,
		Metadata: message.Metadata{
			"eventType": "TaskCancelled",
			"taskID":    formatTaskID(taskID),
		},
	}

	return ep.publisher.Publish("task.cancelled", msg)
}

// Helpers for converting task types to event types
func convertAuthConfig(auth *AuthConfig) *events.AuthConfig {
	if auth == nil {
		return nil
	}
	return &events.AuthConfig{
		Type:     auth.Type,
		Username: auth.Username,
		Password: auth.Password,
		Token:    auth.Token,
		Headers:  auth.Headers,
	}
}

func convertDownloadOptions(opts *DownloadOptions) *events.DownloadOptions {
	if opts == nil {
		return nil
	}
	return &events.DownloadOptions{
		Concurrency: opts.Concurrency,
		MaxSpeed:    opts.MaxSpeed,
		MaxRetries:  opts.MaxRetries,
		Timeout:     opts.Timeout,
	}
}

func generateUUID() string {
	return uuid.New().String()
}

func formatTaskID(taskID uint64) string {
	return strconv.FormatUint(taskID, 10)
}

// ErrOutboxNoSubscribers is returned by an OutboxTransport that requires a live
// subscriber when the destination topic has none yet. The relay keeps the row
// unsent and republishes it on a later pass.
var ErrOutboxNoSubscribers = errors.New("outbox: no subscribers on topic")

// OutboxTransport delivers a built outbox message on the underlying bus. It is
// the explicit seam for the "publish only when a subscriber exists" contract:
// an implementation either requires a subscriber (RequireSubscriberTransport)
// or retains the message regardless (RetainingTransport). The task package
// depends on this interface, not on a concrete transport or a runtime type
// assertion, so every message.Publisher keeps the same semantics.
type OutboxTransport interface {
	// Deliver publishes msg on topic. It returns ErrOutboxNoSubscribers when
	// the transport requires a live subscriber and none exists (the row must
	// stay unsent); nil on a successful publish; any other error is a
	// transport failure.
	Deliver(topic string, msg *message.Message) error
}

// RequiringSubscriberPublisher is a transport that publishes only when a
// subscriber exists and reports the absence atomically (the in-memory bus).
type RequiringSubscriberPublisher interface {
	PublishRequiringSubscriber(topic string, msgs ...*message.Message) error
}

// RequireSubscriberTransport adapts a RequiringSubscriberPublisher (the
// in-memory bus) to OutboxTransport: Deliver returns ErrOutboxNoSubscribers
// when the topic has no subscriber yet.
type RequireSubscriberTransport struct {
	pub RequiringSubscriberPublisher
}

// NewRequireSubscriberTransport builds a RequireSubscriberTransport.
func NewRequireSubscriberTransport(pub RequiringSubscriberPublisher) *RequireSubscriberTransport {
	return &RequireSubscriberTransport{pub: pub}
}

// Deliver implements OutboxTransport.
func (t *RequireSubscriberTransport) Deliver(topic string, msg *message.Message) error {
	if err := t.pub.PublishRequiringSubscriber(topic, msg); err != nil {
		// The only error this returns is "no subscriber": keep the row unsent.
		return fmt.Errorf("%w: %s", ErrOutboxNoSubscribers, topic)
	}
	return nil
}

// RetainingTransport adapts a message.Publisher that retains messages without a
// live subscriber (Kafka) to OutboxTransport: Deliver always publishes.
type RetainingTransport struct {
	pub message.Publisher
}

// NewRetainingTransport builds a RetainingTransport.
func NewRetainingTransport(pub message.Publisher) *RetainingTransport {
	return &RetainingTransport{pub: pub}
}

// Deliver implements OutboxTransport.
func (t *RetainingTransport) Deliver(topic string, msg *message.Message) error {
	return t.pub.Publish(topic, msg)
}

// outboxTopic maps an outbox event type to the topic it is published on. The
// existing transport and topics are kept; OutboxEvent.Destination is recorded
// for later work and does not affect routing yet.
func outboxTopic(eventType string) (string, bool) {
	switch eventType {
	case OutboxEventTaskCreated:
		return "task.created", true
	default:
		return "", false
	}
}

// PublishOutboxEvent publishes a stored outbox event on the existing
// transport. The message UUID is the outbox event id, so a re-publish after a
// crash carries the same UUID. Delivery is at-least-once; consumers do not
// dedupe on the UUID yet.
func (ep *Publisher) PublishOutboxEvent(_ context.Context, event OutboxEvent) error {
	if ep == nil || ep.outbox == nil {
		return errors.New("outbox: no transport configured")
	}

	topic, ok := outboxTopic(event.EventType)
	if !ok {
		return fmt.Errorf("outbox: unknown event type %q", event.EventType)
	}

	msg := &message.Message{
		UUID:    event.ID,
		Payload: event.Payload,
		Metadata: message.Metadata{
			"eventType": event.EventType,
			"taskID":    formatTaskID(event.AggregateID),
		},
	}

	return ep.outbox.Deliver(topic, msg)
}
