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
