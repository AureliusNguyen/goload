package task

import (
	"context"
	"time"
)

// ReconcileCandidate is a task that should be running but has no live
// attempt: it needs a new dispatch.
type ReconcileCandidate struct {
	TaskID uint64
	// Generation is the task's current attempt generation. It grows with
	// every claim, so the reconciler can tell a task that was tried again
	// from one that has been waiting unchanged since its last dispatch.
	Generation uint64
}

// ReconcileRepository finds the tasks the reconciler must dispatch again.
type ReconcileRepository interface {
	// ListReconcilable returns the tasks that ClaimTask would accept at now,
	// in ascending id order, starting after afterID, up to limit:
	//
	//   - PENDING, unowned and due (next_attempt_at is NULL or not after now);
	//   - DOWNLOADING or STORING whose lease is missing or expired (a lease
	//     is expired from the instant now reaches lease_expires_at).
	//
	// PAUSED, CANCELLED, COMPLETED and FAILED tasks and retries that are not
	// due yet are never returned. A limit that is zero or negative returns no
	// candidates and no error.
	ListReconcilable(ctx context.Context, now time.Time, afterID uint64, limit int) ([]ReconcileCandidate, error)
}
