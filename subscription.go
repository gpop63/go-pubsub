package pubsub

import (
	"context"
	"sync/atomic"
)

// Subscription receives messages matching a topic pattern on the channel
// returned by C.
type Subscription[T any] struct {
	pattern      pattern
	broker       *Broker[T]
	filter       func(Message[T]) bool
	ch           chan Message[T] // internal send channel (or filter input)
	out          chan Message[T] // external read channel (equals ch when no filter)
	closed       bool
	dropped      uint64 // messages dropped due to full buffer
	filterPanics uint64 // messages dropped due to filter panic
}

// C returns a receive-only channel for messages.
// Closed when the subscription or broker is closed.
func (s *Subscription[T]) C() <-chan Message[T] {
	return s.out
}

// Close unsubscribes from the broker. Safe to call more than once.
func (s *Subscription[T]) Close() error {
	return s.broker.Unsubscribe(s)
}

// Pattern returns the topic pattern this subscription matches.
func (s *Subscription[T]) Pattern() string {
	return s.pattern.raw
}

// Dropped returns the number of messages dropped due to a full buffer.
func (s *Subscription[T]) Dropped() uint64 {
	return atomic.LoadUint64(&s.dropped)
}

// FilterPanics returns the number of messages dropped because the filter
// panicked. Non-zero means the filter has a bug.
func (s *Subscription[T]) FilterPanics() uint64 {
	return atomic.LoadUint64(&s.filterPanics)
}

// startFilter runs the filter predicate in a goroutine, forwarding
// matching messages from ch to out. When ctx is cancelled it drains
// buffered messages and exits. Filter panics are recovered.
func (s *Subscription[T]) startFilter(ctx context.Context) {
	go func() {
		defer close(s.out)

		for {
			select {
			case msg, ok := <-s.ch:
				if !ok {
					return
				}
				if s.applyFilter(msg) {
					select {
					case s.out <- msg:
					default:
						atomic.AddUint64(&s.dropped, 1)
					}
				}

			case <-ctx.Done():
				// Context cancelled — drain what's left.
				for {
					select {
					case msg, ok := <-s.ch:
						if !ok {
							return
						}
						if s.applyFilter(msg) {
							select {
							case s.out <- msg:
							default:
								atomic.AddUint64(&s.dropped, 1)
							}
						}
					default:
						// Nothing left; unsubscribe so the broker stops
						// sending to this subscription.
						_ = s.broker.Unsubscribe(s)
						return
					}
				}
			}
		}
	}()
}

// applyFilter calls the filter and recovers from any panic.
func (s *Subscription[T]) applyFilter(msg Message[T]) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			atomic.AddUint64(&s.filterPanics, 1)
			ok = false
		}
	}()
	return s.filter(msg)
}
