package redact

import (
	"bytes"
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
