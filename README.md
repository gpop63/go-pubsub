# go-pubsub

[![CI](https://github.com/gpop63/go-pubsub/actions/workflows/ci.yml/badge.svg)](https://github.com/gpop63/go-pubsub/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gpop63/go-pubsub.svg)](https://pkg.go.dev/github.com/gpop63/go-pubsub)

A generic, in-process publish-subscribe message broker for Go with trie-based topic routing and wildcard matching.

## Features

- **Generic messages** — `Broker[T]` works with any payload type
- **Trie-based routing** — efficient topic matching via a prefix tree
- **Wildcard subscriptions** — `*` (single level) and `**` (multi-level)
- **Parallel publishes** — `sync.RWMutex` allows concurrent reads; publishes never block each other
- **Message filtering** — per-subscription predicates with panic recovery
- **Per-subscription observability** — `Dropped()` and `FilterPanics()` counters
- **Context-aware API** — all operations accept `context.Context`
- **Zero dependencies** — standard library only

## Install

```bash
go get github.com/gpop63/go-pubsub
```

## Quick start

```go
package main

import (
    "context"
    "fmt"

    "github.com/gpop63/go-pubsub"
)

func main() {
    ctx := context.Background()
    b := pubsub.NewBroker[string]()
    defer b.Close()

    sub, _ := b.Subscribe(ctx, "orders.*")
    defer sub.Close()

    b.Publish(ctx, "orders.created", "order-123")

    msg := <-sub.C()
    fmt.Println(msg.Payload) // order-123
}
```

## Wildcards

Topics use dot-separated segments. Wildcards can appear in subscription patterns:

| Pattern | Matches | Does not match |
|---------|---------|----------------|
| `orders.created` | `orders.created` | `orders.updated` |
| `orders.*` | `orders.created`, `orders.updated` | `orders.eu.created` |
| `orders.**` | `orders.created`, `orders.eu.created` | `orders` |
| `metrics.*.host1` | `metrics.cpu.host1` | `metrics.cpu.host2` |

- `*` matches **exactly one** segment
- `**` matches **one or more** segments (must be the last segment in a pattern)

## Message filtering

Apply a per-subscription filter that runs in a dedicated goroutine, keeping the broker's publish path unblocked:

```go
sub, _ := b.Subscribe(ctx, "numbers.*", pubsub.WithFilter(func(msg pubsub.Message[int]) bool {
    return msg.Payload%2 == 0 // only even numbers
}))
```

Panics in filter functions are recovered — the message is silently dropped and the subscription continues operating. Use `sub.FilterPanics()` to detect buggy filters.

## Configuration

```go
// Broker-level buffer size (default: 256)
b := pubsub.NewBroker[string](pubsub.WithBufferSize(1024))

// Per-subscription buffer size override
sub, _ := b.Subscribe(ctx, "topic", pubsub.WithSubscriptionBufferSize[string](10))
```

When a subscriber's buffer is full, messages are dropped (not blocked) to protect publisher throughput. Dropped message counts are tracked both globally via `Stats()` and per-subscription via `sub.Dropped()`.

## Architecture

```
Publish("metrics.cpu.host1", val)
          │
          ▼
    ┌─────────┐
    │  root    │
    └────┬────┘
         │
    ┌────▼────┐
    │ metrics │
    └────┬────┘
         │
    ┌────▼────┐     ┌────────┐
    │  cpu    │     │   **   │ ← multi-level wildcard subscribers
    └────┬────┘     └────────┘
         │
    ┌────▼────┐     ┌────────┐
    │ host1   │     │   *    │ ← single-level wildcard subscribers
    └────┬────┘     └────────┘
         │
    [subscribers]
```

The trie structure allows the broker to efficiently route messages by walking the topic segments, checking exact matches and wildcards at each level. This avoids iterating over all subscriptions for every publish.

**Concurrency model:** Publishes acquire a read lock (`RLock`), so multiple goroutines can publish simultaneously. Only subscribe/unsubscribe operations require a write lock, minimizing contention on the hot path.

## API

| Method | Description |
|--------|-------------|
| `NewBroker[T](opts...)` | Create a new broker |
| `Publish(ctx, topic, payload)` | Publish a message (assigns ID + timestamp automatically) |
| `Subscribe(ctx, pattern, opts...)` | Subscribe to a topic pattern |
| `Unsubscribe(sub)` | Remove a subscription |
| `sub.Close()` | Convenience for `Unsubscribe` |
| `sub.C()` | Read-only channel for receiving messages |
| `sub.Pattern()` | Returns the subscription's topic pattern |
| `sub.Dropped()` | Count of messages dropped (full buffer) |
| `sub.FilterPanics()` | Count of filter panics (indicates filter bugs) |
| `Stats(ctx)` | Point-in-time broker statistics |
| `Close()` | Shut down the broker |
