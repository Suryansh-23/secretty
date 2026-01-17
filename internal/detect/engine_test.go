package detect

import (
	"strings"
	"testing"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/types"
)

func TestRegexRuleMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg)
	key := "0x" + strings.Repeat("a", 64)
	input := []byte("PRIVATE_KEY=" + key)
	matches := engine.Find(input)
	if len(matches) != 1 {
		t.Fatalf("matches = %d", len(matches))
	}
	m := matches[0]
	if m.Action != types.ActionMask {
		t.Fatalf("action = %q", m.Action)
	}
	if string(input[m.Start:m.End]) != key {
		t.Fatalf("match text = %q", string(input[m.Start:m.End]))
	}
}

func TestTypedDetectorRequiresAllowBare(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Rulesets.Web3.AllowBare64Hex = false
	engine := NewEngine(cfg)
	key := strings.Repeat("a", 64)
	matches := engine.Find([]byte(key))
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(matches))
	}
}

func TestTypedDetectorAccepts0x(t *testing.T) {
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg)
	key := "0x" + strings.Repeat("b", 64)
	matches := engine.Find([]byte("key=" + key))
	if len(matches) == 0 {
		t.Fatalf("expected match")
	}
}

func TestOverlapResolutionPrefersTyped(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Rulesets.Web3.AllowBare64Hex = true
	engine := NewEngine(cfg)
	key := strings.Repeat("c", 64)
	input := []byte("PRIVATE_KEY=" + key)
	matches := engine.Find(input)
	if len(matches) != 1 {
		t.Fatalf("matches = %d", len(matches))
	}
	if matches[0].RuleName != "evm_private_key" {
		t.Fatalf("expected typed detector to win, got %q", matches[0].RuleName)
	}
}
