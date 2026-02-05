package main

import (
	"os"
	"strings"

	"github.com/suryansh-23/secretty/internal/config"
)

func resolveConfigPath(override string) (string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		return override, nil
	}
	if env := strings.TrimSpace(os.Getenv("SECRETTY_CONFIG")); env != "" {
		return env, nil
	}
	return config.DefaultPath()
}

func exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
