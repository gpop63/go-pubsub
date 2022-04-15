package pubsub

import (
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, err := b.Subscribe("foo.bar")
	if err != nil {
		t.Fatal(err)
	}

	if err := b.Publish("foo.bar", "hello"); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-sub.C():
		if got.Payload != "hello" {
			t.Fatalf("expected payload %q, got %q", "hello", got.Payload)
		}
		if got.ID == "" {
			t.Fatal("expected non-empty message ID")
		}
		if got.CreatedAt.IsZero() {
			t.Fatal("expected non-zero CreatedAt")
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive message")
	}
}

func TestPublishNoMatch(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("foo.bar")

	b.Publish("baz.qux", "miss")

	select {
	case msg := <-sub.C():
		t.Fatalf("unexpected message: %v", msg)
	case <-time.After(50 * time.Millisecond):
		// expected — no match
	}
}

func TestBrokerClose(t *testing.T) {
	b := NewBroker[string]()
	sub1, _ := b.Subscribe("a.b")
	sub2, _ := b.Subscribe("x.y")

	b.Close()

	if _, ok := <-sub1.C(); ok {
		t.Fatal("sub1 channel not closed")
	}
	if _, ok := <-sub2.C(); ok {
		t.Fatal("sub2 channel not closed")
	}
}
