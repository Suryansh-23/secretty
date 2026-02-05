package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suryansh-23/secretty/internal/types"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestParseCanonicalConfig(t *testing.T) {
	path := filepath.Join("testdata", "canonical.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical config: %v", err)
	}
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("parse canonical config: %v", err)
	}

	if cfg.Version != DefaultConfigVersion {
		t.Fatalf("version = %d, want %d", cfg.Version, DefaultConfigVersion)
	}
	if cfg.Mode != types.ModeStrict {
		t.Fatalf("mode = %q, want %q", cfg.Mode, types.ModeStrict)
	}
	if cfg.Redaction.RollingWindowBytes != 32768 {
		t.Fatalf("rolling_window_bytes = %d", cfg.Redaction.RollingWindowBytes)
	}
	if cfg.Overrides.CopyWithoutRender.TTLSeconds != 30 {
		t.Fatalf("ttl_seconds = %d", cfg.Overrides.CopyWithoutRender.TTLSeconds)
	}
	if cfg.Overrides.CopyWithoutRender.Backend != "auto" {
		t.Fatalf("backend = %q", cfg.Overrides.CopyWithoutRender.Backend)
	}
	if len(cfg.Rules) != 10 {
		t.Fatalf("rules count = %d", len(cfg.Rules))
	}
	if len(cfg.TypedDetectors) != 1 {
		t.Fatalf("typed_detectors count = %d", len(cfg.TypedDetectors))
	}
}

func TestValidationRejectsUnknownMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = "unknown"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for unknown mode")
	}
}

func TestParseAppliesDefaults(t *testing.T) {
	data := []byte("version: 1\nmode: strict\n")
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("parse minimal config: %v", err)
	}
	if cfg.Redaction.PlaceholderTemplate != defaultPlaceholderTemplate {
		t.Fatalf("placeholder_template not default")
	}
	if cfg.Masking.BlockChar != defaultBlockChar {
		t.Fatalf("block_char not default")
	}
	if cfg.Masking.Style != types.MaskStyleGlow {
		t.Fatalf("masking.style not default")
	}
}

func TestValidationRejectsInvalidAllowlistEntry(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Allowlist.Commands = []string{""}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for empty allowlist entry")
	}
	cfg.Allowlist.Commands = []string{"["}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid pattern")
	}
}
