package ui

import (
	"testing"

	"github.com/suryansh-23/secretty/internal/types"
)

func TestStatusLineSingle(t *testing.T) {
	line := StatusLine(1, false, false, types.SecretEvmPrivateKey, 0)
	if line != "secretty: redacted EVM_PK" {
		t.Fatalf("line = %q", line)
	}
}

func TestStatusLineStrictWithID(t *testing.T) {
	line := StatusLine(1, true, true, types.SecretEvmPrivateKey, 7)
	if line != "secretty(strict): redacted EVM_PK#7" {
		t.Fatalf("line = %q", line)
	}
}

func TestStatusLineMultiple(t *testing.T) {
	line := StatusLine(3, false, false, types.SecretType(""), 0)
	if line != "secretty: redacted 3 secrets" {
		t.Fatalf("line = %q", line)
	}
}
