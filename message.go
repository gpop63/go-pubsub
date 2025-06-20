package pubsub

import (
	"strconv"
	"sync/atomic"
	"time"
)

// Message represents a published message with a generic payload.
type Message[T any] struct {
	ID        string
	Topic     string
	Payload   T
	CreatedAt time.Time
}

var nextID atomic.Uint64

// generateID returns a unique monotonic message ID.
func generateID() string {
	return strconv.FormatUint(nextID.Add(1), 10)
}
