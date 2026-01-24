package redact

import (
	"bytes"
	"testing"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/types"
)

type matchDetector struct {
	matches []Match
}

func (d matchDetector) Find(text []byte) []Match {
	return d.matches
}

func TestStreamAvoidsSplitMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 4
	var out bytes.Buffer
	detector := matchDetector{matches: []Match{{Start: 1, End: 3, Action: types.ActionMask}}}

	stream := NewStream(&out, cfg, detector, nil, nil)
	_, err := stream.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.String() != "a" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestStreamNoBuffer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	var out bytes.Buffer
	detector := matchDetector{matches: []Match{{Start: 0, End: 3, Action: types.ActionMask}}}

	stream := NewStream(&out, cfg, detector, nil, nil)
	_, err := stream.Write([]byte("abc"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected output with no buffering")
	}
}

func TestStreamNoBufferPreservesUTF8(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	var out bytes.Buffer

	stream := NewStream(&out, cfg, matchDetector{}, nil, nil)
	text := []byte("λ")
	_, err := stream.Write(text[:1])
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output for partial rune")
	}
	_, err = stream.Write(text[1:])
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.String() != "λ" {
		t.Fatalf("output = %q", out.String())
	}
}
