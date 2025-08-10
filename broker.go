package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// BrokerStats is a point-in-time snapshot of broker activity.
type BrokerStats struct {
	Delivered   int64
	Dropped     int64
	Subscribers int
}

// node is a trie node used for topic-based routing.
// Each level of the trie corresponds to one dot-separated segment.
type node[T any] struct {
	children map[string]*node[T]
	subs     []*Subscription[T]
}

func (n *node[T]) isEmpty() bool {
	return len(n.subs) == 0 && len(n.children) == 0
}

// Broker is an in-process pub/sub message broker. It routes messages to
// subscribers using a trie, supporting * and ** wildcards.
// Safe for concurrent use.
type Broker[T any] struct {
	config brokerConfig
	root   *node[T]
	mu     sync.RWMutex
	closed bool

	// Metrics
	delivered   atomic.Int64
	dropped     atomic.Int64
	subscribers atomic.Int64
}

// NewBroker creates a new Broker with the given options.
func NewBroker[T any](opts ...Option) *Broker[T] {
	cfg := defaultConfig()
	for _, op := range opts {
		op(&cfg)
	}

	return &Broker[T]{
		config: cfg,
		root:   &node[T]{children: map[string]*node[T]{}},
	}
}

// Publish sends a message to all subscribers whose patterns match the topic.
// An ID and timestamp are assigned automatically.
func (b *Broker[T]) Publish(ctx context.Context, topic string, payload T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	segments, err := validateTopic(topic)
	if err != nil {
		return err
	}

	msg := Message[T]{
		ID:        generateID(),
		Topic:     topic,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return ErrBrokerClosed
	}

	b.deliverToMatching(b.root, segments, msg)
	return nil
}

// deliverToMatching walks the trie and delivers msg to all matching subscribers.
func (b *Broker[T]) deliverToMatching(n *node[T], levels []string, msg Message[T]) {
	if n == nil {
		return
	}

	// Reached the end of the topic — deliver to subscribers here
	if len(levels) == 0 {
		b.deliver(n.subs, msg)
		return
	}

	part := levels[0]
	rest := levels[1:]

	// Exact match
	if child := n.children[part]; child != nil {
		b.deliverToMatching(child, rest, msg)
	}

	// Single-level wildcard: * matches exactly one segment
	if child := n.children["*"]; child != nil {
		b.deliverToMatching(child, rest, msg)
	}

	// Multi-level wildcard: ** matches one or more remaining segments
	if child := n.children["**"]; child != nil {
		b.deliver(child.subs, msg)
	}
}

// deliver sends msg to each active subscription, dropping on full buffer.
func (b *Broker[T]) deliver(subs []*Subscription[T], msg Message[T]) {
	for _, sub := range subs {
		if sub.closed.Load() {
			continue
		}

		select {
		case sub.ch <- msg:
			b.delivered.Add(1)
		default:
			sub.dropped.Add(1)
			b.dropped.Add(1)
		}
	}
}

// Subscribe creates a subscription for the given topic pattern.
// See the package doc for wildcard syntax.
//
// When a [WithFilter] option is provided, ctx also controls the filter
// goroutine: canceling it unsubscribes, drains already-buffered messages,
// and stops the goroutine.
// Without a filter, the caller must call
// [Subscription.Close] or [Broker.Unsubscribe] to clean up.
func (b *Broker[T]) Subscribe(ctx context.Context, topicPattern string, opts ...SubscribeOption[T]) (*Subscription[T], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pat, err := compilePattern(topicPattern)
	if err != nil {
		return nil, err
	}

	cfg := defaultSubscribeConfig[T]()
	for _, op := range opts {
		op(&cfg)
	}

	bufSize := b.config.bufferSize
	if cfg.bufferSize >= 0 {
		bufSize = cfg.bufferSize
	}

	sub := &Subscription[T]{
		pattern: pat,
		broker:  b,
		ch:      make(chan Message[T], bufSize),
	}

	if cfg.filter != nil {
		sub.filter = cfg.filter
		sub.out = make(chan Message[T], bufSize)
	} else {
		sub.out = sub.ch
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(sub.ch)
		return nil, ErrBrokerClosed
	}
	insertNode(b.root, pat.segments, sub)
	b.subscribers.Add(1)
	b.mu.Unlock()

	// Start the filter after insertion so its cleanup (Unsubscribe on
	// ctx cancellation) can find the subscription in the trie.
	if sub.filter != nil {
		sub.startFilter(ctx)
	}

	return sub, nil
}

func insertNode[T any](n *node[T], levels []string, sub *Subscription[T]) {
	if len(levels) == 0 {
		n.subs = append(n.subs, sub)
		return
	}

	lvl := levels[0]
	if n.children == nil {
		n.children = map[string]*node[T]{}
	}

	child, ok := n.children[lvl]
	if !ok {
		child = &node[T]{children: map[string]*node[T]{}}
		n.children[lvl] = child
	}

	insertNode(child, levels[1:], sub)
}

// Unsubscribe removes a subscription and closes its channel.
// Subsequent calls are no-ops.
func (b *Broker[T]) Unsubscribe(sub *Subscription[T]) error {
	if sub == nil {
		return ErrNilSubscription
	}
	if sub.broker != b {
		return ErrForeignSubscription
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if sub.closed.Swap(true) {
		return nil // already closed
	}

	removeSub(b.root, sub.pattern.segments, sub)
	b.subscribers.Add(-1)
	close(sub.ch)
	return nil
}

// removeSub removes a subscription from the trie and prunes empty nodes.
func removeSub[T any](n *node[T], levels []string, sub *Subscription[T]) bool {
	if n == nil {
		return false
	}

	if len(levels) == 0 {
		for i, s := range n.subs {
			if s != sub {
				continue
			}
			last := len(n.subs) - 1
			n.subs[i] = n.subs[last]
			n.subs[last] = nil // nil the vacated slot to avoid memory leak
			n.subs = n.subs[:last]
			return true
		}
		return false
	}

	lvl := levels[0]
	child, ok := n.children[lvl]
	if !ok {
		return false
	}

	if removeSub(child, levels[1:], sub) {
		// Prune empty trie nodes on the way back up
		if child.isEmpty() {
			delete(n.children, lvl)
		}
		return true
	}
	return false
}

// Stats returns a point-in-time snapshot of broker statistics.
func (b *Broker[T]) Stats(ctx context.Context) (BrokerStats, error) {
	if err := ctx.Err(); err != nil {
		return BrokerStats{}, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return BrokerStats{}, ErrBrokerClosed
	}

	return BrokerStats{
		Delivered:   b.delivered.Load(),
		Dropped:     b.dropped.Load(),
		Subscribers: int(b.subscribers.Load()),
	}, nil
}

// Close shuts down the broker and closes all subscription channels.
// Further operations return ErrBrokerClosed.
func (b *Broker[T]) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}
	b.closed = true

	var closeSubs func(n *node[T])
	closeSubs = func(n *node[T]) {
		if n == nil {
			return
		}
		for _, sub := range n.subs {
			if sub.closed.Swap(true) {
				continue
			}
			close(sub.ch)
		}
		for _, child := range n.children {
			closeSubs(child)
		}
	}

	closeSubs(b.root)
	b.root = nil
	return nil
}
