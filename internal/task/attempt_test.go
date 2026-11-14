package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apperrors "github.com/yuisofull/goload/internal/errors"
	"github.com/yuisofull/goload/internal/storage"
)

var attemptNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func attemptTime(offset time.Duration) *time.Time {
	at := attemptNow.Add(offset)
	return &at
}

// pendingState is a fresh, unowned task with the default retry budget.
func pendingState() *AttemptState {
	return &AttemptState{TaskID: 42, Status: StatusPending, MaxRetries: 3, FileName: "file.bin"}
}

// withStatus is pendingState in another status (still unowned).
func withStatus(status TaskStatus) *AttemptState {
	st := pendingState()
	st.Status = status
	return st
}

// ownedState is a task claimed by worker-a as generation 1 whose lease is
// valid for one more minute at attemptNow.
func ownedState(status TaskStatus) *AttemptState {
	return &AttemptState{
		TaskID:         42,
		Status:         status,
		OwnerID:        "worker-a",
		Generation:     1,
		LeaseExpiresAt: attemptTime(time.Minute),
		MaxRetries:     3,
		FileName:       "file.bin",
	}
}

func attemptResultFixture() AttemptResult {
	return AttemptResult{
		FileName:    "resolved.bin",
		FileSize:    1000,
		ContentType: "application/octet-stream",
		Checksum:    &ChecksumInfo{ChecksumType: "md5", ChecksumValue: "abc123"},
		StorageType: storage.TypeLocal,
		StorageKey:  "42/1/resolved-0011223344556677.bin",
	}
}

func TestDefaultRetryBackoff(t *testing.T) {
	require.Equal(t, 5*time.Second, DefaultRetryBackoff(0))
	require.Equal(t, 5*time.Second, DefaultRetryBackoff(1))
	require.Equal(t, 10*time.Second, DefaultRetryBackoff(2))
	require.Equal(t, 20*time.Second, DefaultRetryBackoff(3))
	require.Equal(t, 160*time.Second, DefaultRetryBackoff(6))
	require.Equal(t, 5*time.Minute, DefaultRetryBackoff(7))
	require.Equal(t, 5*time.Minute, DefaultRetryBackoff(1000))
}

func TestAttemptState_Claim_PendingTask(t *testing.T) {
	st := pendingState()
	st.Progress = DownloadProgress{Progress: 40, DownloadedBytes: 400, TotalBytes: 1000}

	require.NoError(t, st.claim("worker-a", attemptNow, time.Minute))

	require.Equal(t, StatusDownloading, st.Status)
	require.Equal(t, "worker-a", st.OwnerID)
	require.Equal(t, uint64(1), st.Generation)
	require.Equal(t, attemptTime(time.Minute), st.LeaseExpiresAt)
	require.Nil(t, st.NextAttemptAt)
	require.Equal(t, DownloadProgress{}, st.Progress, "every attempt downloads from zero")
	require.Empty(t, st.StoragePath, "claiming must not attach a storage key")
}

func TestAttemptState_Claim_RejectsStatesThatAreNotClaimable(t *testing.T) {
	notDue := pendingState()
	notDue.RetryCount = 1
	notDue.NextAttemptAt = attemptTime(time.Nanosecond)

	stillOwned := pendingState()
	stillOwned.OwnerID = "worker-x"

	for name, st := range map[string]*AttemptState{
		"completed":                     withStatus(StatusCompleted),
		"cancelled":                     withStatus(StatusCancelled),
		"paused":                        withStatus(StatusPaused),
		"failed with retries exhausted": withStatus(StatusFailed),
		"pending retry not yet due":     notDue,
		"pending but still owned":       stillOwned,
		"downloading with a live lease": ownedState(StatusDownloading),
		"storing with a live lease":     ownedState(StatusStoring),
	} {
		t.Run(name, func(t *testing.T) {
			before := *st

			err := st.claim("worker-b", attemptNow, time.Minute)

			require.ErrorIs(t, err, ErrTaskNotClaimable)
			require.True(t, apperrors.IsError(err, apperrors.ErrCodeInvalidState))
			require.Equal(t, before, *st, "a rejected claim must not change the state")
		})
	}
}

func TestAttemptState_Claim_RetryIsClaimableOnceDue(t *testing.T) {
	st := pendingState()
	st.RetryCount = 1
	st.NextAttemptAt = attemptTime(0) // due exactly now

	require.NoError(t, st.claim("worker-a", attemptNow, time.Minute))

	require.Equal(t, uint64(1), st.Generation)
	require.Equal(t, 1, st.RetryCount, "claiming does not reset the retry count")
	require.Nil(t, st.NextAttemptAt)
}

func TestAttemptState_Claim_ExpiredLeaseGetsNewGeneration(t *testing.T) {
	for _, status := range []TaskStatus{StatusDownloading, StatusStoring} {
		t.Run(string(status), func(t *testing.T) {
			st := ownedState(status)
			st.LeaseExpiresAt = attemptTime(0) // expires exactly now
			st.Progress = DownloadProgress{Progress: 80, DownloadedBytes: 800, TotalBytes: 1000}

			require.NoError(t, st.claim("worker-b", attemptNow, time.Minute))

			require.Equal(t, StatusDownloading, st.Status)
			require.Equal(t, "worker-b", st.OwnerID)
			require.Equal(t, uint64(2), st.Generation)
			require.Equal(t, attemptTime(time.Minute), st.LeaseExpiresAt)
			require.Equal(t, DownloadProgress{}, st.Progress)
		})
	}
}

func TestAttemptState_Claim_DownloadingWithoutLeaseIsClaimable(t *testing.T) {
	// A row put into DOWNLOADING by the legacy unfenced status update has no
	// owner and no lease. It must not be stuck forever.
	st := withStatus(StatusDownloading)

	require.NoError(t, st.claim("worker-a", attemptNow, time.Minute))

	require.Equal(t, uint64(1), st.Generation)
	require.Equal(t, "worker-a", st.OwnerID)
}

func TestAttemptState_Renew_ExtendsLeaseAndAppliesUpdate(t *testing.T) {
	st := ownedState(StatusDownloading)
	later := attemptNow.Add(30 * time.Second)
	progress := DownloadProgress{Progress: 25, DownloadedBytes: 250, TotalBytes: 1000}

	require.NoError(t, st.renew("worker-a", 1, later, time.Minute, LeaseUpdate{Progress: &progress, Storing: true}))

	wantLease := later.Add(time.Minute)
	require.Equal(t, &wantLease, st.LeaseExpiresAt)
	require.Equal(t, StatusStoring, st.Status)
	require.Equal(t, progress, st.Progress)
	require.Equal(t, uint64(1), st.Generation, "renewing never changes the generation")

	// A plain heartbeat keeps status and progress.
	require.NoError(t, st.renew("worker-a", 1, later, time.Minute, LeaseUpdate{}))
	require.Equal(t, StatusStoring, st.Status)
	require.Equal(t, progress, st.Progress)
}

// fencedOps are the attempt-originated mutations. Each must pass the same
// ownership check before it changes anything.
var fencedOps = map[string]func(st *AttemptState, workerID string, generation uint64, now time.Time) error{
	"renew": func(st *AttemptState, workerID string, generation uint64, now time.Time) error {
		progress := DownloadProgress{Progress: 99, DownloadedBytes: 990, TotalBytes: 1000}
		return st.renew(workerID, generation, now, time.Minute, LeaseUpdate{Progress: &progress, Storing: true})
	},
	"complete": func(st *AttemptState, workerID string, generation uint64, now time.Time) error {
		_, err := st.complete(workerID, generation, now, attemptResultFixture())
		return err
	},
	"fail": func(st *AttemptState, workerID string, generation uint64, now time.Time) error {
		return st.fail(workerID, generation, now, "boom", DefaultRetryBackoff)
	},
	"release": func(st *AttemptState, workerID string, generation uint64, now time.Time) error {
		return st.release(workerID, generation, now)
	},
}

func TestAttemptState_FencedOps_RejectStaleAttempts(t *testing.T) {
	// unowned is a task that left the owned states after generation 1 held it.
	unowned := func(status TaskStatus) func() *AttemptState {
		return func() *AttemptState {
			st := withStatus(status)
			st.Generation = 1
			return st
		}
	}
	storing := func() *AttemptState { return ownedState(StatusStoring) }
	newerGeneration := func() *AttemptState {
		st := ownedState(StatusStoring)
		st.OwnerID = "worker-b"
		st.Generation = 2
		return st
	}

	cases := []struct {
		name       string
		state      func() *AttemptState
		workerID   string
		generation uint64
		now        time.Time
	}{
		{"another worker", storing, "worker-b", 1, attemptNow},
		{"another generation of the same worker", storing, "worker-a", 2, attemptNow},
		{"a replaced attempt", newerGeneration, "worker-a", 1, attemptNow},
		{"a lease that expires exactly now", storing, "worker-a", 1, attemptNow.Add(time.Minute)},
		{"a task back in PENDING", unowned(StatusPending), "worker-a", 1, attemptNow},
		{"a paused task", unowned(StatusPaused), "worker-a", 1, attemptNow},
		{"a cancelled task", unowned(StatusCancelled), "worker-a", 1, attemptNow},
		{"a failed task", unowned(StatusFailed), "worker-a", 1, attemptNow},
	}

	for opName, op := range fencedOps {
		for _, tc := range cases {
			t.Run(opName+" by "+tc.name, func(t *testing.T) {
				st := tc.state()
				before := *st

				err := op(st, tc.workerID, tc.generation, tc.now)

				require.ErrorIs(t, err, ErrAttemptRejected)
				require.True(t, apperrors.IsError(err, apperrors.ErrCodeConflict))
				require.Equal(t, before, *st, "a rejected call must not change the state")
			})
		}
	}
}

func TestAttemptState_Complete_AttachesResultAndClearsOwnership(t *testing.T) {
	st := ownedState(StatusStoring)
	previous := "previous failure"
	st.ErrorMessage = &previous
	result := attemptResultFixture()

	alreadyDone, err := st.complete("worker-a", 1, attemptNow, result)

	require.NoError(t, err)
	require.False(t, alreadyDone)
	require.Equal(t, StatusCompleted, st.Status)
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Nil(t, st.NextAttemptAt)
	require.Equal(t, uint64(1), st.Generation, "the winning generation stays on the row")
	require.Equal(t, "resolved.bin", st.FileName)
	require.Equal(t, storage.TypeLocal, st.StorageType)
	require.Equal(t, result.StorageKey, st.StoragePath)
	require.Equal(t, result.Checksum, st.Checksum)
	require.Equal(t, DownloadProgress{Progress: 100, DownloadedBytes: 1000, TotalBytes: 1000}, st.Progress)
	require.Nil(t, st.ErrorMessage)
	require.Equal(t, attemptTime(0), st.CompletedAt)
}

func TestAttemptState_Complete_RequiresStoring(t *testing.T) {
	st := ownedState(StatusDownloading)
	before := *st

	alreadyDone, err := st.complete("worker-a", 1, attemptNow, attemptResultFixture())

	require.ErrorIs(t, err, ErrAttemptRejected)
	require.False(t, alreadyDone)
	require.Equal(t, before, *st)
}

func TestAttemptState_Complete_RetryOfTheWinningAttemptIsAlreadyDone(t *testing.T) {
	st := ownedState(StatusStoring)
	result := attemptResultFixture()
	_, err := st.complete("worker-a", 1, attemptNow, result)
	require.NoError(t, err)
	committed := *st

	// The response was lost and the worker retries long after its lease would
	// have expired. The retry-safe case is evaluated before any lease check.
	alreadyDone, err := st.complete("worker-a", 1, attemptNow.Add(time.Hour), result)

	require.NoError(t, err)
	require.True(t, alreadyDone)
	require.Equal(t, committed, *st, "the retry must not rewrite the committed result")
}

func TestAttemptState_Complete_RetryRejectsOtherGenerationOrResult(t *testing.T) {
	otherKey := attemptResultFixture()
	otherKey.StorageKey = "42/2/resolved-0011223344556677.bin"
	otherChecksum := attemptResultFixture()
	otherChecksum.Checksum = &ChecksumInfo{ChecksumType: "md5", ChecksumValue: "different"}
	noChecksum := attemptResultFixture()
	noChecksum.Checksum = nil
	noName := attemptResultFixture()
	noName.FileName = ""
	otherName := attemptResultFixture()
	otherName.FileName = "other.bin"

	for name, tc := range map[string]struct {
		generation uint64
		result     AttemptResult
	}{
		"a different generation":  {generation: 2, result: attemptResultFixture()},
		"a different storage key": {generation: 1, result: otherKey},
		"a different checksum":    {generation: 1, result: otherChecksum},
		"an omitted checksum":     {generation: 1, result: noChecksum},
		"an omitted file name":    {generation: 1, result: noName},
		"a different file name":   {generation: 1, result: otherName},
	} {
		t.Run(name, func(t *testing.T) {
			st := ownedState(StatusStoring)
			_, err := st.complete("worker-a", 1, attemptNow, attemptResultFixture())
			require.NoError(t, err)
			before := *st

			alreadyDone, err := st.complete("worker-a", tc.generation, attemptNow, tc.result)

			require.ErrorIs(t, err, ErrAttemptRejected)
			require.False(t, alreadyDone)
			require.Equal(t, before, *st)
		})
	}
}

func TestAttemptState_Fail_RetriesRemain(t *testing.T) {
	st := ownedState(StatusDownloading)

	require.NoError(t, st.fail("worker-a", 1, attemptNow, "connection reset", DefaultRetryBackoff))

	require.Equal(t, StatusPending, st.Status)
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Equal(t, 1, st.RetryCount)
	require.Equal(t, attemptTime(5*time.Second), st.NextAttemptAt)
	require.NotNil(t, st.ErrorMessage)
	require.Equal(t, "connection reset", *st.ErrorMessage)
	require.Equal(t, uint64(1), st.Generation)
	require.Empty(t, st.StoragePath, "a failed attempt must not attach a storage key")
}

func TestAttemptState_Fail_LastRetryIsStillARetry(t *testing.T) {
	st := ownedState(StatusStoring)
	st.RetryCount = 2 // MaxRetries is 3: this failure uses the third retry

	require.NoError(t, st.fail("worker-a", 1, attemptNow, "connection reset", DefaultRetryBackoff))

	require.Equal(t, StatusPending, st.Status)
	require.Equal(t, 3, st.RetryCount)
	require.Equal(t, attemptTime(20*time.Second), st.NextAttemptAt)
}

func TestAttemptState_Fail_RetriesExhaustedIsTerminal(t *testing.T) {
	st := ownedState(StatusStoring)
	st.RetryCount = 3 // MaxRetries is 3: this is the fourth failed attempt

	require.NoError(t, st.fail("worker-a", 1, attemptNow, "still broken", DefaultRetryBackoff))

	require.Equal(t, StatusFailed, st.Status)
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Equal(t, 4, st.RetryCount)
	require.Nil(t, st.NextAttemptAt)
	require.NotNil(t, st.ErrorMessage)
	require.Equal(t, "still broken", *st.ErrorMessage)
}

func TestAttemptState_Fail_ZeroMaxRetriesIsTerminal(t *testing.T) {
	st := ownedState(StatusDownloading)
	st.MaxRetries = 0

	require.NoError(t, st.fail("worker-a", 1, attemptNow, "boom", DefaultRetryBackoff))

	require.Equal(t, StatusFailed, st.Status)
	require.Equal(t, 1, st.RetryCount)
	require.Nil(t, st.NextAttemptAt)
}

func TestAttemptState_Release_ReturnsTaskToPending(t *testing.T) {
	st := ownedState(StatusStoring)
	st.RetryCount = 2

	require.NoError(t, st.release("worker-a", 1, attemptNow))

	require.Equal(t, StatusPending, st.Status)
	require.Empty(t, st.OwnerID)
	require.Nil(t, st.LeaseExpiresAt)
	require.Nil(t, st.NextAttemptAt, "a released task is claimable immediately")
	require.Equal(t, 2, st.RetryCount, "a release is not a failure")

	require.NoError(t, st.claim("worker-b", attemptNow, time.Minute))
	require.Equal(t, uint64(2), st.Generation)
}

func TestAttemptState_Cancel(t *testing.T) {
	waitingRetry := pendingState()
	waitingRetry.RetryCount = 1
	waitingRetry.NextAttemptAt = attemptTime(time.Minute)

	for name, st := range map[string]*AttemptState{
		"pending":       pendingState(),
		"pending retry": waitingRetry,
		"downloading":   ownedState(StatusDownloading),
		"storing":       ownedState(StatusStoring),
		"paused":        withStatus(StatusPaused),
		"failed":        withStatus(StatusFailed),
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, st.cancel())

			require.Equal(t, StatusCancelled, st.Status)
			require.Empty(t, st.OwnerID)
			require.Nil(t, st.LeaseExpiresAt)
			require.Nil(t, st.NextAttemptAt)
		})
	}

	for _, status := range []TaskStatus{StatusCompleted, StatusCancelled} {
		t.Run("rejects "+string(status), func(t *testing.T) {
			st := withStatus(status)

			err := st.cancel()

			require.True(t, apperrors.IsError(err, apperrors.ErrCodeInvalidInput))
			require.Equal(t, status, st.Status)
		})
	}
}

func TestAttemptState_Pause(t *testing.T) {
	st := ownedState(StatusDownloading)

	require.NoError(t, st.pause())

	require.Equal(t, StatusPaused, st.Status)
	require.Empty(t, st.OwnerID, "pausing retires the attempt")
	require.Nil(t, st.LeaseExpiresAt)
	require.Equal(t, uint64(1), st.Generation)

	for _, status := range []TaskStatus{
		StatusPending, StatusStoring, StatusCompleted, StatusFailed, StatusCancelled, StatusPaused,
	} {
		other := ownedState(status)
		err := other.pause()
		require.True(t, apperrors.IsError(err, apperrors.ErrCodeInvalidInput), "pause from %s", status)
		require.Equal(t, status, other.Status)
		require.Equal(t, "worker-a", other.OwnerID)
	}
}

func TestAttemptState_Resume(t *testing.T) {
	st := withStatus(StatusPaused)
	st.Generation = 1

	require.NoError(t, st.resume())

	require.Equal(t, StatusPending, st.Status)
	require.NoError(t, st.claim("worker-b", attemptNow, time.Minute), "a resumed task is claimable as a new attempt")
	require.Equal(t, uint64(2), st.Generation)

	notPaused := pendingState()
	require.True(t, apperrors.IsError(notPaused.resume(), apperrors.ErrCodeInvalidInput))
	require.Equal(t, StatusPending, notPaused.Status)
}

func TestAttemptState_Retry(t *testing.T) {
	st := withStatus(StatusFailed)
	st.RetryCount = 4
	st.Generation = 4

	require.NoError(t, st.retry())

	require.Equal(t, StatusPending, st.Status)
	require.Zero(t, st.RetryCount, "a user retry grants a fresh retry budget")
	require.Nil(t, st.NextAttemptAt)
	require.NoError(t, st.claim("worker-a", attemptNow, time.Minute))
	require.Equal(t, uint64(5), st.Generation)

	notFailed := pendingState()
	require.True(t, apperrors.IsError(notFailed.retry(), apperrors.ErrCodeInvalidInput))
	require.Equal(t, StatusPending, notFailed.Status)
}
