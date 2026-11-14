package task

import (
	"context"
	"sync"

	apperrors "github.com/yuisofull/goload/internal/errors"
)

// memAttempts is an in-memory AttemptRepository for service unit tests. It
// does not model transactions or locking; fakeTxManager runs the callback
// inline.
type memAttempts struct {
	mu     sync.Mutex
	states map[uint64]AttemptState
}

func newMemAttempts(states ...AttemptState) *memAttempts {
	m := &memAttempts{states: make(map[uint64]AttemptState, len(states))}
	for _, st := range states {
		m.states[st.TaskID] = st
	}
	return m
}

func (m *memAttempts) GetAttemptState(_ context.Context, taskID uint64) (*AttemptState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[taskID]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return &st, nil
}

func (m *memAttempts) LockAttemptState(ctx context.Context, taskID uint64) (*AttemptState, error) {
	return m.GetAttemptState(ctx, taskID)
}

func (m *memAttempts) SaveAttemptState(_ context.Context, state *AttemptState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state.TaskID] = *state
	return nil
}
