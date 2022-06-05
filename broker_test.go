package pubsub

import (
	"errors"
	"testing"
	"time"
)

// --- Basic pub/sub ---

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

// --- Wildcard matching ---

func TestSingleLevelWildcard(t *testing.T) {
	b := NewBroker[int]()
	defer b.Close()

	sub, _ := b.Subscribe("foo.*")

	b.Publish("foo.bar", 42)

	select {
	case msg := <-sub.C():
		if msg.Payload != 42 {
			t.Fatalf("unexpected payload: %d", msg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive message")
	}
}

func TestSingleLevelWildcardNoMatchDeeper(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("foo.*")

	b.Publish("foo.bar.baz", "deep")

	select {
	case msg := <-sub.C():
		t.Fatalf("single-level wildcard should not match deeper topic, got: %v", msg)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestMultiLevelWildcard(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("foo.**")

	b.Publish("foo.bar.baz", "ok")

	select {
	case msg := <-sub.C():
		if msg.Topic != "foo.bar.baz" {
			t.Fatalf("unexpected topic: %s", msg.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive message")
	}
}

func TestMultiLevelWildcardMatchesSingleLevel(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("foo.**")

	b.Publish("foo.bar", "one-level")

	select {
	case msg := <-sub.C():
		if msg.Payload != "one-level" {
			t.Fatalf("unexpected payload: %s", msg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("** should match one level deep")
	}
}

func TestMultiLevelWildcardNoMatchRoot(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("foo.**")

	b.Publish("foo", "root")

	select {
	case msg := <-sub.C():
		t.Fatalf("** should not match zero additional levels, got: %v", msg)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestWildcardMixed(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("a.*.c")

	b.Publish("a.b.c", "match")
	b.Publish("a.x.c", "also-match")
	b.Publish("a.b.d", "no-match")

	received := 0
	timeout := time.After(time.Second)
	for received < 2 {
		select {
		case <-sub.C():
			received++
		case <-timeout:
			t.Fatalf("expected 2 messages, got %d", received)
		}
	}

	select {
	case msg := <-sub.C():
		t.Fatalf("unexpected message: %v", msg)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestCatchAllDoubleStarPattern(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, err := b.Subscribe("**")
	if err != nil {
		t.Fatal(err)
	}

	topics := []string{"foo", "foo.bar", "a.b.c.d"}
	for _, topic := range topics {
		b.Publish(topic, topic)
	}

	for i, want := range topics {
		select {
		case msg := <-sub.C():
			if msg.Payload != want {
				t.Fatalf("message %d: expected payload %q, got %q", i, want, msg.Payload)
			}
		case <-time.After(time.Second):
			t.Fatalf("expected message %d (%q), timed out", i, want)
		}
	}
}

func TestMultipleSubscribersSamePattern(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub1, _ := b.Subscribe("events.click")
	sub2, _ := b.Subscribe("events.click")

	b.Publish("events.click", "btn")

	for i, sub := range []*Subscription[string]{sub1, sub2} {
		select {
		case msg := <-sub.C():
			if msg.Payload != "btn" {
				t.Fatalf("sub%d: expected payload %q, got %q", i+1, "btn", msg.Payload)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub%d: did not receive message", i+1)
		}
	}
}

// --- Pattern validation ---

func TestSubscribeInvalidPatterns(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	tests := []struct {
		name    string
		pattern string
	}{
		{"empty", ""},
		{"double dot", "a..b"},
		{"leading dot", ".a.b"},
		{"trailing dot", "a.b."},
		{"double star not last", "a.**.b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := b.Subscribe(tt.pattern)
			if err == nil {
				t.Fatalf("expected error for pattern %q", tt.pattern)
			}
		})
	}
}

func TestPublishInvalidTopics(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	tests := []struct {
		name  string
		topic string
	}{
		{"empty", ""},
		{"double dot", "a..b"},
		{"wildcard star", "foo.*"},
		{"wildcard double star", "foo.**"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.Publish(tt.topic, "payload")
			if err == nil {
				t.Fatalf("expected error for topic %q", tt.topic)
			}
		})
	}
}

// --- Unsubscribe ---

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("x.y")
	b.Unsubscribe(sub)

	_, ok := <-sub.C()
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestUnsubscribeIdempotent(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("x.y")

	if err := b.Unsubscribe(sub); err != nil {
		t.Fatal(err)
	}
	if err := b.Unsubscribe(sub); err != nil {
		t.Fatal("second unsubscribe should be a no-op")
	}
}

func TestSubscriptionClose(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("x.y")
	sub.Close()

	_, ok := <-sub.C()
	if ok {
		t.Fatal("expected channel to be closed after sub.Close()")
	}
}

func TestPublishAfterUnsubscribe(t *testing.T) {
	b := NewBroker[int]()
	defer b.Close()

	sub, _ := b.Subscribe("a.b")
	b.Unsubscribe(sub)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("publish panicked: %v", r)
		}
	}()

	b.Publish("a.b", 1)
}

func TestUnsubscribePrunesTrie(t *testing.T) {
	b := NewBroker[string]()
	defer b.Close()

	sub, _ := b.Subscribe("a.b.c.d")
	b.Unsubscribe(sub)

	b.mu.RLock()
	_, exists := b.root.children["a"]
	b.mu.RUnlock()

	if exists {
		t.Fatal("expected trie node 'a' to be pruned after last subscription removed")
	}
}

// --- Broker close ---

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

func TestBrokerCloseIdempotent(t *testing.T) {
	b := NewBroker[string]()

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("expected nil on repeated Close, got %v", err)
	}
}

func TestPublishAfterClose(t *testing.T) {
	b := NewBroker[string]()
	b.Close()

	err := b.Publish("foo.bar", "late")
	if !errors.Is(err, ErrBrokerClosed) {
		t.Fatalf("expected ErrBrokerClosed, got %v", err)
	}
}

func TestSubscribeAfterClose(t *testing.T) {
	b := NewBroker[string]()
	b.Close()

	_, err := b.Subscribe("foo.bar")
	if !errors.Is(err, ErrBrokerClosed) {
		t.Fatalf("expected ErrBrokerClosed, got %v", err)
	}
}
