package debug

import (
	"fmt"
	"io"
	"os"
)

// Logger provides minimal sanitized logging hooks.
type Logger struct {
	enabled bool
	out     io.Writer
}

// New returns a logger writing to stderr when enabled.
func New(enabled bool) *Logger {
	return &Logger{enabled: enabled, out: os.Stderr}
}

// Infof writes a formatted log line when enabled.
func (l *Logger) Infof(format string, args ...any) {
	if l == nil || !l.enabled {
		return
	}
	_, _ = fmt.Fprintf(l.out, format+"\n", args...)
}
