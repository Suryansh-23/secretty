package redact

import "github.com/suryansh-23/secretty/internal/types"

// Match defines a redaction span.
type Match struct {
	Start      int
	End        int
	Action     types.Action
	SecretType types.SecretType
	RuleName   string
	ID         int
}

// Detector finds redaction matches in text buffers.
type Detector interface {
	Find(text []byte) []Match
}

// NoopDetector performs no detection.
type NoopDetector struct{}

// Find returns no matches.
func (NoopDetector) Find(text []byte) []Match {
	return nil
}
