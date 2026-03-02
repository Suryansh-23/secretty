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

type testPauseGate struct {
	active bool
}

func (p *testPauseGate) IsPausedNow() bool {
	return p.active
}

func TestStreamPausePassThroughAndResume(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 0
	cfg.Masking.Style = types.MaskStyleBlock
	cfg.Masking.BlockChar = "#"
	cfg.Rulesets.Web3.Enabled = true

	out := &bytes.Buffer{}
	pause := &testPauseGate{}
	stream := redact.NewStream(out, cfg, detect.NewEngine(cfg), nil, nil, pause)

	pause.active = true
	secret := "PRIVATE_KEY=0x" + strings.Repeat("a", 64) + "\n"
	if _, err := stream.Write([]byte(secret)); err != nil {
		t.Fatalf("write while paused: %v", err)
	}

	if !strings.Contains(out.String(), strings.Repeat("a", 16)) {
		t.Fatalf("expected unredacted output while paused, got %q", out.String())
	}

	pause.active = false
	out.Reset()
	if _, err := stream.Write([]byte(secret)); err != nil {
		t.Fatalf("write after resume: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if strings.Contains(out.String(), strings.Repeat("a", 16)) {
		t.Fatalf("expected redacted output after resume, got %q", out.String())
	}
}

func TestStreamPauseFlushesPendingBeforeBypass(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.RollingWindowBytes = 64
	cfg.Masking.Style = types.MaskStyleBlock
	cfg.Masking.BlockChar = "#"
	cfg.Rulesets.Web3.Enabled = true

	out := &bytes.Buffer{}
	pause := &testPauseGate{}
	stream := redact.NewStream(out, cfg, detect.NewEngine(cfg), nil, nil, pause)

	secret := "PRIVATE_KEY=0x" + strings.Repeat("a", 64)
	if _, err := stream.Write([]byte(secret)); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	pause.active = true
	if _, err := stream.Write([]byte("\n")); err != nil {
		t.Fatalf("pause write: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got := out.String()
	if strings.Contains(got, strings.Repeat("a", 16)) {
		t.Fatalf("expected pending buffered secret to flush redacted before pause bypass, got %q", got)
	}
}
