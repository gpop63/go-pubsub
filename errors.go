package pubsub

import "errors"

// Sentinel errors returned by the broker.
var (
	// ErrBrokerClosed is returned when an operation is attempted on a closed broker.
	ErrBrokerClosed = errors.New("pubsub: broker is closed")

	// ErrTopicEmpty is returned when a publish or subscribe topic is empty.
	ErrTopicEmpty = errors.New("pubsub: topic must not be empty")

	// ErrInvalidPattern is returned when a subscribe pattern is malformed.
	ErrInvalidPattern = errors.New("pubsub: invalid topic pattern")

	// ErrNilSubscription is returned when a nil subscription is passed to Unsubscribe.
	ErrNilSubscription = errors.New("pubsub: nil subscription")

	// ErrForeignSubscription is returned when a subscription from a different broker
	// is passed to Unsubscribe.
	ErrForeignSubscription = errors.New("pubsub: subscription belongs to a different broker")
)
