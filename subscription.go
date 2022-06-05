package pubsub

// Subscription receives messages matching a topic pattern on the channel
// returned by C.
type Subscription[T any] struct {
	pattern pattern
	broker  *Broker[T]
	ch      chan Message[T]
	closed  bool
}

// C returns a receive-only channel for messages.
// Closed when the subscription or broker is closed.
func (s *Subscription[T]) C() <-chan Message[T] {
	return s.ch
}

// Close unsubscribes from the broker. Safe to call more than once.
func (s *Subscription[T]) Close() error {
	return s.broker.Unsubscribe(s)
}
