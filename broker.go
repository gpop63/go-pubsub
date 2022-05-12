package pubsub

import (
	"sync"
	"time"
)

// node is a trie node used for topic-based routing.
// Each level of the trie corresponds to one dot-separated segment.
type node[T any] struct {
	children map[string]*node[T]
	subs     []*Subscription[T]
}

// Broker is an in-process pub/sub message broker. It routes messages to
// subscribers using a trie, supporting * and ** wildcards.
// Safe for concurrent use.
type Broker[T any] struct {
	root *node[T]
	mu   sync.RWMutex
}

// NewBroker creates a new Broker.
func NewBroker[T any]() *Broker[T] {
	return &Broker[T]{
		root: &node[T]{children: map[string]*node[T]{}},
	}
}

// Publish sends a message to all subscribers whose patterns match the topic.
func (b *Broker[T]) Publish(topic string, payload T) error {
	segments, err := validateTopic(topic)
	if err != nil {
		return err
	}

	id, err := generateID() // crypto/rand syscall on every publish
	if err != nil {
		return err
	}

	msg := Message[T]{
		ID:        id,
		Topic:     topic,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

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
		for _, sub := range n.subs {
			sub.ch <- msg
		}
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
		for _, sub := range child.subs {
			sub.ch <- msg
		}
	}
}

// Subscribe creates a subscription for the given topic pattern.
func (b *Broker[T]) Subscribe(topicPattern string) (*Subscription[T], error) {
	pat, err := compilePattern(topicPattern)
	if err != nil {
		return nil, err
	}

	sub := &Subscription[T]{
		pattern: pat,
		broker:  b,
		ch:      make(chan Message[T], 256),
	}

	b.mu.Lock()
	insertNode(b.root, pat.segments, sub)
	b.mu.Unlock()

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

// Close shuts down the broker and closes all subscription channels.
func (b *Broker[T]) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var closeSubs func(n *node[T])
	closeSubs = func(n *node[T]) {
		if n == nil {
			return
		}
		for _, sub := range n.subs {
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
