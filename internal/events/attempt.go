package events

import (
	"errors"
	"time"

	"github.com/yuisofull/goload/internal/storage"
)

// The attempt contract: the types and sentinel errors the download worker and
// the task service's attempt (ownership) API share. Neither service imports
// the other; both import this package.

var (
	// ErrTaskNotClaimable is the cause of a rejected ClaimTask: the task is
	// terminal, paused, failed, waiting for a retry that is not due yet, or
	// leased by a live attempt. The worker drops the delivery.
	ErrTaskNotClaimable = errors.New("task is not claimable")
	// ErrAttemptRejected is the cause of every fencing rejection: the caller
	// is not the current owner and generation, its lease expired, or the task
	// left the owned states. The worker must stop and never force its result.
	ErrAttemptRejected = errors.New("attempt rejected")
)

// Attempt is one successful claim: the fencing token and its lease.
type Attempt struct {
	TaskID         uint64
	WorkerID       string
	Generation     uint64
	LeaseExpiresAt time.Time
}

// AttemptProgress is the progress a heartbeat reports.
type AttemptProgress struct {
	Progress        float64
	DownloadedBytes int64
	TotalBytes      int64
}

// LeaseUpdate is what a heartbeat may carry besides the renewal itself.
type LeaseUpdate struct {
	// Progress, when set, replaces the task's progress.
	Progress *AttemptProgress
	// Storing reports that the attempt began writing to storage
	// (DOWNLOADING -> STORING). It is required before CompleteAttempt.
	Storing bool
}

// AttemptResult is what a finished attempt reports to CompleteAttempt.
type AttemptResult struct {
	FileName    string
	FileSize    int64
	ContentType string
	Checksum    *ChecksumInfo
	StorageType storage.Type
	// StorageKey is the attempt's own generation-scoped storage key.
	StorageKey string
}
