package task

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/yuisofull/goload/internal/errors"
)

// AttemptService is the ownership API download workers use. Workers never
// touch task rows directly; every call carries an explicit worker id (a
// stable identity generated once per worker process) and, after the claim,
// the attempt generation, which is the fencing token.
//
// Except for the retry-safe completion case, every mutating call is rejected
// with ErrAttemptRejected unless worker id and generation match the row and
// the lease is unexpired. An attempt whose lease expired must ClaimTask again
// and gets a new generation; it can never revive its old lease.
type AttemptService interface {
	// ClaimTask claims a claimable task: PENDING, unowned and due, or
	// DOWNLOADING/STORING with an expired lease. Anything else is rejected
	// with ErrTaskNotClaimable.
	ClaimTask(ctx context.Context, taskID uint64, workerID string) (*Attempt, error)
	// RenewLease is the heartbeat. It extends the lease and may carry a
	// progress report and the DOWNLOADING -> STORING step. It returns the new
	// lease expiry.
	RenewLease(
		ctx context.Context,
		taskID uint64,
		workerID string,
		generation uint64,
		update LeaseUpdate,
	) (time.Time, error)
	// CompleteAttempt commits the attempt's result: status COMPLETED, file
	// name, checksum and the winning storage key in one transaction, together
	// with the lifecycle completion event in the outbox. Repeating the call
	// for the winning generation with the same result returns nil without
	// writing anything.
	CompleteAttempt(
		ctx context.Context,
		taskID uint64,
		workerID string,
		generation uint64,
		result AttemptResult,
	) error
	// FailAttempt records a failed attempt: back to PENDING with a retry time
	// while the retry budget lasts, terminal FAILED once it is exhausted.
	FailAttempt(ctx context.Context, taskID uint64, workerID string, generation uint64, cause error) error
	// ReleaseAttempt hands the task back without counting a failure. The task
	// is claimable again immediately.
	ReleaseAttempt(ctx context.Context, taskID uint64, workerID string, generation uint64) error
}

type attemptService struct {
	attempts AttemptRepository
	tx       TxManager
	outbox   OutboxRepository
	now      func() time.Time
	lease    time.Duration
	backoff  func(retryCount int) time.Duration
}

// AttemptServiceOption configures an AttemptService.
type AttemptServiceOption func(*attemptService)

// WithAttemptClock overrides the clock that decides lease expiry and retry
// due times. A nil clock is ignored.
func WithAttemptClock(now func() time.Time) AttemptServiceOption {
	return func(s *attemptService) {
		if now != nil {
			s.now = now
		}
	}
}

// WithLeaseDuration overrides DefaultLeaseDuration. Values that are zero or
// negative are ignored.
func WithLeaseDuration(d time.Duration) AttemptServiceOption {
	return func(s *attemptService) {
		if d > 0 {
			s.lease = d
		}
	}
}

// WithRetryBackoff overrides DefaultRetryBackoff. A nil function is ignored.
func WithRetryBackoff(backoff func(retryCount int) time.Duration) AttemptServiceOption {
	return func(s *attemptService) {
		if backoff != nil {
			s.backoff = backoff
		}
	}
}

// NewAttemptService creates the ownership API on top of the task tables.
// tx must be the TxManager of the same database as attempts and outbox.
func NewAttemptService(
	attempts AttemptRepository,
	tx TxManager,
	outbox OutboxRepository,
	opts ...AttemptServiceOption,
) AttemptService {
	s := &attemptService{
		attempts: attempts,
		tx:       tx,
		outbox:   outbox,
		now:      func() time.Time { return time.Now().UTC() },
		lease:    DefaultLeaseDuration,
		backoff:  DefaultRetryBackoff,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func errWorkerIDRequired() error {
	return &errors.Error{Code: errors.ErrCodeInvalidInput, Message: "worker id is required"}
}

// lockAttemptState loads a task's ownership state under the row lock of the
// transaction carried by ctx and maps repository errors to service errors.
func lockAttemptState(ctx context.Context, attempts AttemptRepository, taskID uint64) (*AttemptState, error) {
	st, err := attempts.LockAttemptState(ctx, taskID)
	if err != nil {
		if stderrors.Is(err, errors.ErrNotFound) {
			return nil, &errors.Error{Code: errors.ErrCodeNotFound, Message: "Task not found", Cause: err}
		}
		return nil, &errors.Error{Code: errors.ErrCodeInternal, Message: "failed to load task ownership", Cause: err}
	}
	return st, nil
}

// saveAttemptState writes st back on the transaction carried by ctx.
func saveAttemptState(ctx context.Context, attempts AttemptRepository, st *AttemptState) error {
	if err := attempts.SaveAttemptState(ctx, st); err != nil {
		return &errors.Error{Code: errors.ErrCodeInternal, Message: "failed to save task ownership", Cause: err}
	}
	return nil
}

// transition runs one state transition in one transaction: lock the row,
// apply fn, and save when fn asks for it. An error from fn rolls back, so a
// rejected call writes nothing.
func (s *attemptService) transition(
	ctx context.Context,
	taskID uint64,
	fn func(ctx context.Context, st *AttemptState) (save bool, err error),
) error {
	return s.tx.DoInTx(ctx, func(ctx context.Context) error {
		st, err := lockAttemptState(ctx, s.attempts, taskID)
		if err != nil {
			return err
		}
		save, err := fn(ctx, st)
		if err != nil || !save {
			return err
		}
		return saveAttemptState(ctx, s.attempts, st)
	})
}

func (s *attemptService) ClaimTask(ctx context.Context, taskID uint64, workerID string) (*Attempt, error) {
	if workerID == "" {
		return nil, errWorkerIDRequired()
	}

	var attempt *Attempt
	err := s.transition(ctx, taskID, func(_ context.Context, st *AttemptState) (bool, error) {
		if err := st.claim(workerID, s.now(), s.lease); err != nil {
			return false, err
		}
		attempt = &Attempt{
			TaskID:         taskID,
			WorkerID:       workerID,
			Generation:     st.Generation,
			LeaseExpiresAt: *st.LeaseExpiresAt,
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return attempt, nil
}

func (s *attemptService) RenewLease(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
	update LeaseUpdate,
) (time.Time, error) {
	if workerID == "" {
		return time.Time{}, errWorkerIDRequired()
	}

	var lease time.Time
	err := s.transition(ctx, taskID, func(_ context.Context, st *AttemptState) (bool, error) {
		if err := st.renew(workerID, generation, s.now(), s.lease, update); err != nil {
			return false, err
		}
		lease = *st.LeaseExpiresAt
		return true, nil
	})
	if err != nil {
		return time.Time{}, err
	}
	return lease, nil
}

func (s *attemptService) CompleteAttempt(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
	result AttemptResult,
) error {
	if workerID == "" {
		return errWorkerIDRequired()
	}
	if result.StorageKey == "" {
		return &errors.Error{
			Code:    errors.ErrCodeInvalidInput,
			Message: "storage key is required to complete an attempt",
		}
	}
	if result.StorageType == "" {
		return &errors.Error{
			Code:    errors.ErrCodeInvalidInput,
			Message: "storage type is required to complete an attempt",
		}
	}

	return s.transition(ctx, taskID, func(ctx context.Context, st *AttemptState) (bool, error) {
		now := s.now()
		alreadyDone, err := st.complete(workerID, generation, now, result)
		if err != nil {
			return false, err
		}
		if alreadyDone {
			// The first call committed and only its response was lost: report
			// success again without writing a second result or event.
			return false, nil
		}

		// The completion event joins the transaction that completes the task.
		// The outbox relay publishes it after commit.
		event, err := NewTaskCompletedOutboxEvent(taskID, result, now.UTC())
		if err == nil {
			err = s.outbox.InsertUnsent(ctx, event)
		}
		if err != nil {
			return false, &errors.Error{
				Code:    errors.ErrCodeInternal,
				Message: "failed to enqueue task completed event",
				Cause:   err,
			}
		}
		return true, nil
	})
}

// failureMessage is the user-facing text stored for a failed attempt.
func failureMessage(cause error) string {
	if message := userFacingTaskError(cause); message != "" {
		return message
	}
	return "unknown error"
}

func (s *attemptService) FailAttempt(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
	cause error,
) error {
	if workerID == "" {
		return errWorkerIDRequired()
	}

	message := failureMessage(cause)
	return s.transition(ctx, taskID, func(_ context.Context, st *AttemptState) (bool, error) {
		if err := st.fail(workerID, generation, s.now(), message, s.backoff); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (s *attemptService) ReleaseAttempt(
	ctx context.Context,
	taskID uint64,
	workerID string,
	generation uint64,
) error {
	if workerID == "" {
		return errWorkerIDRequired()
	}

	return s.transition(ctx, taskID, func(_ context.Context, st *AttemptState) (bool, error) {
		if err := st.release(workerID, generation, s.now()); err != nil {
			return false, err
		}
		return true, nil
	})
}
