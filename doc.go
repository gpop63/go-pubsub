// Package pubsub is a generic in-process publish-subscribe message broker
// with trie-based topic routing.
//
// Topics are dot-separated (e.g. "orders.created"). Subscription patterns
// support wildcards: * matches one segment, ** matches one or more (must
// be last).
//
//	b := pubsub.NewBroker[string]()
//	defer b.Close()
//
//	sub, _ := b.Subscribe(ctx, "events.*")
//	defer sub.Close()
//
//	b.Publish(ctx, "events.click", "button-1")
//	msg := <-sub.C()
//
// Publish takes a read lock so concurrent publishes don't contend with
// each other; only Subscribe and Unsubscribe take a write lock.
//
// Subscriptions can run a filter predicate in a separate goroutine via
// [WithFilter]. Panics in the filter are recovered. Slow filters won't
// block publishes but will fill the subscription's buffer and cause drops.
//
// Payloads are not copied. If T contains pointers, slices, or maps,
// don't mutate them after publishing.
package pubsub
