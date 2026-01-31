package shellconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveBlockNoFile(t *testing.T) {
	changed, err := RemoveBlock(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("expected no change")
	}
}

func TestRemoveBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shellrc")
	input := "line1\n# >>> secretty >>>\nexport SECRETTY=1\n# <<< secretty <<<\nline2\n"
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	changed, err := RemoveBlock(path)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Fatalf("expected change")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	expected := "line1\nline2\n"
	if string(data) != expected {
		t.Fatalf("output = %q", string(data))
	}
}

func TestInstallBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shellrc")
	changed, err := InstallBlock(path, "zsh", "/tmp/secretty/config.yaml", "/usr/local/bin/secretty")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !changed {
		t.Fatalf("expected change")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "SECRETTY_CONFIG") {
		t.Fatalf("expected config export")
	}
	if !strings.Contains(string(data), "command -v secretty") {
		t.Fatalf("expected command existence check")
	}
	if !strings.Contains(string(data), "exec \"$secretty_bin\"") {
		t.Fatalf("expected exec wrapper")
	}
	if !strings.Contains(string(data), "exec \"$secretty_bin\" </dev/tty >/dev/tty 2>/dev/tty") {
		t.Fatalf("expected tty redirection")
	}
}
