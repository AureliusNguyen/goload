package inmem

import (
	"context"
	"sync"

	"github.com/go-kit/log"

	"github.com/yuisofull/goload/pkg/message"
)

type subscription struct {
	ch     chan *message.Message
	ctx    context.Context
	cancle context.CancelFunc

	// mu makes sends and the channel close mutually exclusive. Senders hold
	// RLock for the duration of a send; shutdown cancels ctx first (which
	// unblocks any pending send), then takes the write lock to close ch.
	mu     sync.RWMutex
	closed bool
}

// send delivers m to the subscriber. It returns false if the subscription is
// closed or its context ends before the message is accepted. It never sends on
// a closed channel.
func (s *subscription) send(m *message.Message) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return false
	}
	select {
	case s.ch <- m:
		return true
	case <-s.ctx.Done():
		return false
	}
}

// shutdown cancels the subscription and closes its channel exactly once, only
// after in-flight sends have returned.
func (s *subscription) shutdown() {
	s.cancle()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

type broker struct {
	mu     sync.RWMutex
	buffer int
	subs   map[string][]*subscription
	logger log.Logger
}

func newBroker(buffer int, logger log.Logger) *broker {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	return &broker{buffer: buffer, subs: make(map[string][]*subscription), logger: logger}
}

func (b *broker) Close() error {
	// best-effort: drop all subs
	b.mu.Lock()
	defer b.mu.Unlock()
	for topic, arr := range b.subs {
		for _, sub := range arr {
			sub.shutdown()
		}
		delete(b.subs, topic)
	}
	return nil
}

func (b *broker) addSub(topic string, s *subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[topic] = append(b.subs[topic], s)
}

func (b *broker) removeSub(topic string, s *subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	arr := b.subs[topic]
	for i := range arr {
		if arr[i] == s {
			b.subs[topic] = append(arr[:i], arr[i+1:]...)
			break
		}
	}
	if len(b.subs[topic]) == 0 {
		delete(b.subs, topic)
	}
}

func (b *broker) getSubs(topic string) []*subscription {
	b.mu.RLock()
	defer b.mu.RUnlock()
	arr := b.subs[topic]
	// return a copy to avoid races
	out := make([]*subscription, len(arr))
	copy(out, arr)
	return out
}

type Subscriber struct {
	b *broker
}

func NewSubscriber(b *broker) *Subscriber {
	return &Subscriber{b: b}
}

func (s *Subscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	ch := make(chan *message.Message, s.b.buffer)
	ctx, cancle := context.WithCancel(ctx)
	sub := &subscription{ch: ch, ctx: ctx, cancle: cancle}
	s.b.addSub(topic, sub)

	// remove subscription when ctx done
	go func() {
		<-ctx.Done()
		s.b.removeSub(topic, sub)
		sub.shutdown()
	}()

	return ch, nil
}

func (s *Subscriber) Close() error {
	// best-effort: drop all subs
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	for topic, arr := range s.b.subs {
		for _, sub := range arr {
			sub.shutdown()
		}
		delete(s.b.subs, topic)
	}
	return nil
}

// helpers to create shared broker + pub/sub
func NewBroker(buffer int, logger log.Logger) *broker { return newBroker(buffer, logger) }

func NewPublisherAndSubscriber(logger log.Logger) (*Publisher, *Subscriber) {
	b := newBroker(100, logger) // default buffer size
	return NewPublisher(b), NewSubscriber(b)
}
