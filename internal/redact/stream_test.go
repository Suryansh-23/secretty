package redact

import (
	"testing"
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
