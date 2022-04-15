package pubsub

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Message represents a published message with a generic payload.
type Message[T any] struct {
	ID        string
	Topic     string
	Payload   T
	CreatedAt time.Time
}

// generateID creates a cryptographically random 16-byte hex-encoded ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate message ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}
