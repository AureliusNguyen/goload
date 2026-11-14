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
}

// NewEventPublisher creates a new event publisher for task service
func NewEventPublisher(publisher message.Publisher) *Publisher {
	return &Publisher{
		publisher: publisher,
	}
}

// PublishTaskCreated publishes a task created event
func (ep *Publisher) PublishTaskCreated(ctx context.Context, task *Task) error {
	event := events.TaskCreatedEvent{
		TaskID:          task.ID,
		OfAccountID:     task.OfAccountID,
		FileName:        task.FileName,
		SourceURL:       task.SourceURL,
		SourceType:      string(task.SourceType),
		SourceAuth:      convertAuthConfig(task.SourceAuth),
		DownloadOptions: convertDownloadOptions(task.DownloadOptions),
		Metadata:        task.Metadata,
		CreatedAt:       task.CreatedAt,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &message.Message{
		UUID:    generateUUID(),
		Payload: payload,
		Metadata: message.Metadata{
			"eventType": "TaskCreated",
			"taskID":    formatTaskID(task.ID),
		},
	}

	return ep.publisher.Publish("task.created", msg)
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

// ErrOutboxNoSubscribers is returned by PublishOutboxEvent when the transport
// reports that nobody is subscribed to the destination topic yet.
var ErrOutboxNoSubscribers = errors.New("outbox: no subscribers on topic")

// subscriberAware is implemented by transports that can tell whether a topic
// currently has subscribers (the in-memory bus). Kafka does not implement it.
type subscriberAware interface {
	HasSubscribers(topic string) bool
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
	if ep == nil || ep.publisher == nil {
		return errors.New("outbox: no message publisher configured")
	}

	topic, ok := outboxTopic(event.EventType)
	if !ok {
		return fmt.Errorf("outbox: unknown event type %q", event.EventType)
	}

	// The in-memory bus drops messages for topics without subscribers. Report
	// that instead, so the caller keeps the event unsent and retries.
	if sa, ok := ep.publisher.(subscriberAware); ok && !sa.HasSubscribers(topic) {
		return fmt.Errorf("%w: %s", ErrOutboxNoSubscribers, topic)
	}

	msg := &message.Message{
		UUID:    event.ID,
		Payload: event.Payload,
		Metadata: message.Metadata{
			"eventType": event.EventType,
			"taskID":    formatTaskID(event.AggregateID),
		},
	}

	return ep.publisher.Publish(topic, msg)
}
