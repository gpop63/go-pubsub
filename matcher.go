package pubsub

import (
	"fmt"
	"strings"
)

// pattern holds a validated topic pattern with pre-split segments.
// Validation happens once at subscribe time.
type pattern struct {
	raw      string
	segments []string
}

// compilePattern validates a topic pattern and splits it into segments.
// See the package doc for wildcard rules.
func compilePattern(raw string) (pattern, error) {
	if raw == "" {
		return pattern{}, ErrTopicEmpty
	}

	segments := strings.Split(raw, ".")

	for i, seg := range segments {
		if seg == "" {
			return pattern{}, fmt.Errorf("%w: empty segment in %q", ErrInvalidPattern, raw)
		}
		if seg == "**" && i != len(segments)-1 {
			return pattern{}, fmt.Errorf("%w: ** must be the last segment in %q", ErrInvalidPattern, raw)
		}
	}

	return pattern{
		raw:      raw,
		segments: segments,
	}, nil
}

// validateTopic checks that a publish topic is well-formed and returns
// the pre-split segments. Publish topics must not contain wildcards.
func validateTopic(topic string) ([]string, error) {
	if topic == "" {
		return nil, ErrTopicEmpty
	}
	segments := strings.Split(topic, ".")
	for _, seg := range segments {
		if seg == "" {
			return nil, fmt.Errorf("%w: empty segment in %q", ErrInvalidPattern, topic)
		}
		if seg == "*" || seg == "**" {
			return nil, fmt.Errorf("%w: wildcards not allowed in publish topic %q", ErrInvalidPattern, topic)
		}
	}
	return segments, nil
}
