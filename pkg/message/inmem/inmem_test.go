package inmem

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/yuisofull/goload/pkg/message"
)

func TestPublishSubscribe_Ack(t *testing.T) {
	pub, sub := NewPublisherAndSubscriber(log.NewNopLogger())
	ctx := t.Context()

	ch, err := sub.Subscribe(ctx, "topic1")
	if err != nil {
		t.Fatal(err)
	}

	msg := message.NewMessage("1", []byte("hello"))

	if err := pub.Publish("topic1", msg); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-ch:
		if m == nil {
			t.Fatal("nil message")
		}
		assert.Equal(t, "1", m.UUID)
		assert.Equal(t, []byte("hello"), []byte(m.Payload))
		m.Ack()

	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestPublishSubscribe_NackRedeliver(t *testing.T) {
	pub, sub := NewPublisherAndSubscriber(log.NewNopLogger())
	ctx := t.Context()

	ch, err := sub.Subscribe(ctx, "topic2")
	if err != nil {
		t.Fatal(err)
	}

	msg := message.NewMessage("2", []byte("world"))

	if err := pub.Publish("topic2", msg); err != nil {
		t.Fatal(err)
	}

	// receive first, Nack -> expect redelivery
	select {
	case m := <-ch:
		if m == nil {
			t.Fatal("nil message")
		}
		m.Nack()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first message")
	}

	select {
	case m := <-ch:
		if m == nil {
			t.Fatal("nil message on redelivery")
		}
		m.Ack()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for redelivered message")
	}
}

func TestPublishRequiringSubscriber(t *testing.T) {
	pub, sub := NewPublisherAndSubscriber(log.NewNopLogger())

	err := pub.PublishRequiringSubscriber("topic3", message.NewMessage("3", []byte("x")))
	assert.True(t, errors.Is(err, ErrNoSubscribers))

	ch, err := sub.Subscribe(t.Context(), "topic3")
	if err != nil {
		t.Fatal(err)
	}

	if err := pub.PublishRequiringSubscriber("topic3", message.NewMessage("3", []byte("x"))); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-ch:
		assert.Equal(t, "3", m.UUID)
		m.Ack()
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestCloseWhilePublishing_NoRace(t *testing.T) {
	for i := 0; i < 50; i++ {
		pub, sub := NewPublisherAndSubscriber(log.NewNopLogger())
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := sub.Subscribe(ctx, "t")
		if err != nil {
			t.Fatal(err)
		}
		// fill the buffer so deliver goroutines block in send
		for j := 0; j < 300; j++ {
			_ = pub.Publish("t", message.NewMessage("x", []byte("x")))
		}
		cancel()
		if i%2 == 0 {
			_ = sub.Close()
		}
		// channel must close (drain until closed)
		timeout := time.After(2 * time.Second)
		for done := false; !done; {
			select {
			case _, ok := <-ch:
				done = !ok
			case <-timeout:
				t.Fatal("channel not closed")
			}
		}
	}
}
