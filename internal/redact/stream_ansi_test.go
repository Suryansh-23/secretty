package redact_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
)

func TestInteractiveAnsiAwareDetection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	cfg.Masking.Style = types.MaskStyleBlock
	cfg.Masking.BlockChar = "#"
	cfg.Rulesets.APIKeys.Enabled = true

	out := &bytes.Buffer{}
	stream := redact.NewStream(out, cfg, detect.NewEngine(cfg), nil, nil)

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

func TestInteractiveSplitWriteRedaction(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	cfg.Masking.Style = types.MaskStyleBlock
	cfg.Masking.BlockChar = "#"
	cfg.Rulesets.Web3.Enabled = true

	out := &bytes.Buffer{}
	stream := redact.NewStream(out, cfg, detect.NewEngine(cfg), nil, nil)

	part1 := []byte("PRIVATE_KEY=\x1b[38;5;81m" + strings.Repeat("a", 12))
	part2 := []byte(strings.Repeat("a", 52) + "\x1b[0m\n")
	if _, err := stream.Write(part1); err != nil {
		t.Fatalf("write part1: %v", err)
	}
	if _, err := stream.Write(part2); err != nil {
		t.Fatalf("write part2: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	output := out.String()
	if strings.Contains(output, strings.Repeat("a", 8)) {
		t.Fatalf("expected private key to be redacted, got %q", output)
	}
	if !strings.Contains(output, "PRIVATE_KEY=") {
		t.Fatalf("expected key label to remain")
	}
}
