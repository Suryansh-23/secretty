package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/suryansh-23/secretty/internal/ui"
)

func currentBadge() ui.Badge {
	return ui.Badge{
		Platform: platformLabel(),
		Shell:    shellLabel(),
	}
}

func platformLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func shellLabel() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return "shell"
	}
	return filepath.Base(shell)
}
