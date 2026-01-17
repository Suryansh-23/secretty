package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	cfg := DefaultConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := Write(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}
