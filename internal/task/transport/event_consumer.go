package tasktransport

import (
	"context"
	"encoding/json"

	"github.com/yuisofull/goload/internal/errors"
	"github.com/yuisofull/goload/internal/events"
	"github.com/yuisofull/goload/internal/storage"
	"github.com/yuisofull/goload/internal/task"
	"github.com/yuisofull/goload/pkg/message"
)

// EventConsumer receives the lifecycle events of download workers:
// task.progress.updated, task.completed and task.failed.
//
// These events are notifications. The task row is written only by the
// ownership API (task.AttemptService), whose calls are checked against the
// attempt generation. An event on the bus carries no generation, so applying
// it would let a stale attempt overwrite the winner. By default the consumer
// therefore acknowledges every event and writes nothing. It stays subscribed
// so that the outbox relay always finds a subscriber for the lifecycle events
// it publishes.
type EventConsumer struct {
	taskService  task.Service
	subscriber   message.Subscriber
	errorHandler func(context.Context, error)
	// legacyStateWrites re-enables the pre-ownership behavior: every event is
	// applied to the task row without a fencing check.
	legacyStateWrites bool
}

// EventConsumerOption configures an EventConsumer.
type EventConsumerOption func(*EventConsumer)

// WithLegacyStateWrites makes the consumer apply lifecycle events to the task
// row, as it did before the ownership API existed.
//
// Legacy: this is the unfenced write path the ownership API replaces. It
// exists only for cmd/task, whose download workers cannot reach the ownership
// API until it is exposed over gRPC, and it goes away with that exposure. Do
// not use it anywhere else.
func WithLegacyStateWrites() EventConsumerOption {
	return func(ec *EventConsumer) { ec.legacyStateWrites = true }
}

// NewEventConsumer creates a new event consumer for task service
func NewEventConsumer(
	taskService task.Service,
	subscriber message.Subscriber,
	errorHandler func(context.Context, error),
	opts ...EventConsumerOption,
) *EventConsumer {
	ec := &EventConsumer{
		taskService:  taskService,
		subscriber:   subscriber,
		errorHandler: errorHandler,
	}
	for _, o := range opts {
		o(ec)
	}
	return ec
}

// Start begins consuming events
func (ec *EventConsumer) Start(ctx context.Context) error {
	progressCh, err := ec.subscriber.Subscribe(ctx, "task.progress.updated")
	if err != nil {
		return err
	}

	completedCh, err := ec.subscriber.Subscribe(ctx, "task.completed")
	if err != nil {
		return err
	}

	failedCh, err := ec.subscriber.Subscribe(ctx, "task.failed")
	if err != nil {
		return err
	}

	// Start goroutines to handle each event type
	go ec.handleProgressUpdates(ctx, progressCh)
	go ec.handleCompletions(ctx, completedCh)
	go ec.handleFailures(ctx, failedCh)

	// Block until context is canceled.
	<-ctx.Done()
	return ctx.Err()
}

// handleProgressUpdates processes progress update messages
func (ec *EventConsumer) handleProgressUpdates(ctx context.Context, ch <-chan *message.Message) {
	ec.consume(ctx, ch, ec.handleTaskProgressUpdated)
}

// handleCompletions processes completion messages
func (ec *EventConsumer) handleCompletions(ctx context.Context, ch <-chan *message.Message) {
	ec.consume(ctx, ch, ec.handleTaskCompleted)
}

// handleFailures processes failure messages
func (ec *EventConsumer) handleFailures(ctx context.Context, ch <-chan *message.Message) {
	ec.consume(ctx, ch, ec.handleTaskFailed)
}

// consume acknowledges every message of one lifecycle topic. Only in legacy
// mode it first applies the event to the task row.
func (ec *EventConsumer) consume(
	ctx context.Context,
	ch <-chan *message.Message,
	apply func(context.Context, *message.Message) error,
) {
	for msg := range ch {
		if !ec.legacyStateWrites {
			// A notification: the task service already committed whatever
			// this event announces, under the attempt's generation.
			msg.Ack()
			continue
		}

		if err := apply(ctx, msg); err != nil {
			if errors.IsError(err, errors.ErrCodeNotFound) {
				msg.Ack()
				continue
			}
			ec.errorHandler(ctx, err)
			msg.Nack()
		} else {
			msg.Ack()
		}
	}
}

// handleTaskProgressUpdated applies a progress update (legacy mode only)
func (ec *EventConsumer) handleTaskProgressUpdated(ctx context.Context, msg *message.Message) error {
	var event events.TaskProgressUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return err
	}

	// Update task progress in the database
	progress := task.DownloadProgress{
		Progress:        event.Progress,
		DownloadedBytes: event.DownloadedBytes,
		TotalBytes:      event.TotalBytes,
	}

	if err := ec.taskService.UpdateTaskProgress(ctx, event.TaskID, progress); err != nil {
		return err
	}

	return nil
}

// handleTaskCompleted applies a completion event (legacy mode only)
func (ec *EventConsumer) handleTaskCompleted(ctx context.Context, msg *message.Message) error {
	var event events.TaskCompletedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return err
	}

	if err := ec.taskService.CompleteTask(ctx, event.TaskID); err != nil {
		return err
	}

	if event.FileName != "" {
		if err := ec.taskService.UpdateFileName(ctx, event.TaskID, event.FileName); err != nil {
			return err
		}
	}

	if event.Checksum != nil {
		checksum := task.ChecksumInfo{
			ChecksumType:  event.Checksum.ChecksumType,
			ChecksumValue: event.Checksum.ChecksumValue,
		}
		if err := ec.taskService.UpdateTaskChecksum(ctx, event.TaskID, &checksum); err != nil {
			return err
		}
	}

	if event.StorageType != "" || event.StorageKey != "" {
		if err := ec.taskService.UpdateStorageInfo(
			ctx,
			event.TaskID,
			storage.TypeValue(event.StorageType),
			event.StorageKey,
		); err != nil {
			return err
		}
	}

	if event.FileSize > 0 {
		if err := ec.taskService.UpdateTaskProgress(ctx, event.TaskID, task.DownloadProgress{
			TotalBytes: event.FileSize,
		}); err != nil {
			return err
		}
	}

	return nil
}

// handleTaskFailed applies a failure event (legacy mode only)
func (ec *EventConsumer) handleTaskFailed(ctx context.Context, msg *message.Message) error {
	var event events.TaskFailedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return err
	}

	// Create error from the event
	taskErr := &errors.Error{
		Code:    errors.ErrCodeInternal,
		Message: event.Error,
	}

	if err := ec.taskService.UpdateTaskError(ctx, event.TaskID, taskErr); err != nil {
		return err
	}

	return nil
}
