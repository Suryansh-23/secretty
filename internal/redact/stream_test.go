package redact

import (
	"bytes"
	"strings"
	"testing"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/types"
)

func TestExtractLabel(t *testing.T) {
	text := []byte("GITHUB_API_KEY=ghp_1234567890abcdef1234567890abcdef1234\n")
	match := Match{Start: 15, End: len(text) - 1}
	label := extractLabel(text, match)
	if label != "GITHUB_API_KEY" {
		t.Fatalf("label=%q", label)
	}
}

func TestExtractLabelMissing(t *testing.T) {
	text := []byte("no label here\n")
	match := Match{Start: 3, End: 5}
	label := extractLabel(text, match)
	if label != "" {
		t.Fatalf("expected empty label, got %q", label)
	}
}

func TestInteractiveAnsiAwareDetection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	cfg.Masking.Style = types.MaskStyleBlock
	cfg.Masking.BlockChar = "#"

	out := &bytes.Buffer{}
	stream := NewStream(out, cfg, detect.NewEngine(cfg), nil, nil)

	input := []byte("GITHUB_API_KEY=\x1b[31mghp_0123456789ABCDEFGHijklmnopqrstuvwx\x1b[0m\n")
	if _, err := stream.Write(input); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "ghp_") {
		t.Fatalf("expected ghp token to be redacted, got %q", output)
	}
	if !strings.Contains(output, "GITHUB_API_KEY=") {
		t.Fatalf("expected key label to remain")
	}
}
