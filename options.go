package pubsub

const defaultBufferSize = 256

type brokerConfig struct {
	bufferSize int
}

func defaultConfig() brokerConfig {
	return brokerConfig{
		bufferSize: defaultBufferSize,
	}
}

// Option configures a [Broker].
type Option func(*brokerConfig)

// WithBufferSize sets the default channel buffer size for new subscriptions.
// A size of 0 means messages are dropped immediately if the subscriber
// is not ready to receive. Negative values are treated as 0.
func WithBufferSize(size int) Option {
	return func(bc *brokerConfig) {
		if size < 0 {
			size = 0
		}
		bc.bufferSize = size
	}
}

type subscribeConfig[T any] struct {
	bufferSize int // -1 means use broker default
	filter     func(Message[T]) bool
}

func defaultSubscribeConfig[T any]() subscribeConfig[T] {
	return subscribeConfig[T]{
		bufferSize: -1,
	}
}

// SubscribeOption configures an individual subscription.
type SubscribeOption[T any] func(*subscribeConfig[T])

// WithSubscriptionBufferSize overrides the broker's default buffer size
// for this subscription. Negative values are treated as 0.
func WithSubscriptionBufferSize[T any](size int) SubscribeOption[T] {
	return func(sc *subscribeConfig[T]) {
		if size < 0 {
			size = 0
		}
		sc.bufferSize = size
	}
}

// WithFilter sets a predicate that messages must pass before delivery.
// The filter runs in its own goroutine, so it won't block publishes.
// Panics are recovered and the message is dropped.
func WithFilter[T any](fn func(Message[T]) bool) SubscribeOption[T] {
	return func(sc *subscribeConfig[T]) {
		sc.filter = fn
	}
}
