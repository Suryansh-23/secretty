package redact

import (
	"bytes"
	"hash/fnv"
	"regexp"
	"strings"
	"testing"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/types"
)

func TestPlaceholderReplacement(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redaction.PlaceholderTemplate = "<<{type}:{id:02d}>>"
	r := NewRedactor(cfg)

	out, err := r.Apply([]byte("secret"), []Match{{
		Start:      0,
		End:        6,
		Action:     types.ActionPlaceholder,
		SecretType: types.SecretEvmPrivateKey,
		ID:         3,
	}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if string(out) != "<<EVM_PK:03>>" {
		t.Fatalf("output = %q", string(out))
	}
}

func TestHexRandomMaskKeepsPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Masking.Style = types.MaskStyleBlock
	r := NewRedactor(cfg)
	r.rng = bytes.NewReader(bytes.Repeat([]byte{0x01}, 128))

	in := []byte("0xabcdef")
	out, err := r.Apply(in, []Match{{
		Start:      0,
		End:        len(in),
		Action:     types.ActionMask,
		SecretType: types.SecretEvmPrivateKey,
	}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("len(out)=%d len(in)=%d", len(out), len(in))
	}
	if string(out[:2]) != "0x" {
		t.Fatalf("prefix = %q", string(out[:2]))
	}
}

func TestGlowMaskKeepsVisualLength(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Masking.Style = types.MaskStyleGlow
	cfg.Masking.BlockChar = "#"
	r := NewRedactor(cfg)

	in := []byte("secret")
	out, err := r.Apply(in, []Match{{
		Start:      0,
		End:        len(in),
		Action:     types.ActionMask,
		SecretType: types.SecretEvmPrivateKey,
	}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	plain := stripANSI(string(out))
	if plain != strings.Repeat("#", len(in)) {
		t.Fatalf("plain output = %q", plain)
	}
	if !strings.Contains(string(out), "\x1b[0m") {
		t.Fatalf("expected ANSI reset")
	}
}

func TestGlowParamsDeterministic(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Masking.Style = types.MaskStyleGlow
	r1 := NewRedactor(cfg)
	r2 := NewRedactor(cfg)

	secret := []byte("tmdb_api_key")
	idx1, band1 := r1.glowParams(secret)
	idx2, band2 := r2.glowParams(secret)
	if idx1 != idx2 || band1 != band2 {
		t.Fatalf("expected deterministic params, got (%d,%d) and (%d,%d)", idx1, band1, idx2, band2)
	}
	expectedIndex, expectedBand := glowParamsForTest(secret)
	if idx1 != expectedIndex || band1 != expectedBand {
		t.Fatalf("params=(%d,%d) expected=(%d,%d)", idx1, band1, expectedIndex, expectedBand)
	}
}

func TestGlowStartIndexAvoidsRepeat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Masking.Style = types.MaskStyleGlow
	r := NewRedactor(cfg)

	secret := []byte("repeat-secret")
	firstIdx, firstBand := r.glowParams(secret)
	secondIdx, secondBand := r.glowParams(secret)
	if len(glowPalette) > 1 && firstIdx == secondIdx && firstBand == secondBand {
		t.Fatalf("expected different params, got (%d,%d) and (%d,%d)", firstIdx, firstBand, secondIdx, secondBand)
	}
}

func TestMorseMaskMatchesLength(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Masking.Style = types.MaskStyleMorse
	cfg.Masking.MorseMessage = "SOS"
	r := NewRedactor(cfg)

	in := []byte("secret")
	out, err := r.Apply(in, []Match{{
		Start:      0,
		End:        len(in),
		Action:     types.ActionMask,
		SecretType: types.SecretEvmPrivateKey,
	}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("len(out)=%d len(in)=%d", len(out), len(in))
	}
}

func stripANSI(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(input, "")
}

func glowParamsForTest(input []byte) (int, int) {
	if len(glowPalette) == 0 {
		return 0, 1
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write(input)
	sum := hasher.Sum32()
	idx := int(sum % uint32(len(glowPalette)))
	bandSize := int((sum>>8)%4) + 2
	if bandSize < 2 {
		bandSize = 2
	}
	return idx, bandSize
}
