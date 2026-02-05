package allowlist

import (
	"path"
	"path/filepath"
	"strings"
)

// Match reports whether a command should bypass redaction based on allowlist entries.
// Entries match against argv0 basename by default. If an entry contains a path
// separator, it is matched against the resolved full path when available.
func Match(entries []string, argv0 string, resolvedPath string) (bool, error) {
	argv0 = strings.TrimSpace(argv0)
	resolvedPath = strings.TrimSpace(resolvedPath)
	base := commandBase(argv0, resolvedPath)
	full := commandFull(argv0, resolvedPath)

	for _, entry := range entries {
		pattern := strings.TrimSpace(entry)
		if pattern == "" {
			continue
		}
		target := base
		if strings.Contains(pattern, "/") {
			target = full
		}
		if target == "" {
			continue
		}
		ok, err := path.Match(pattern, target)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func commandBase(argv0 string, resolvedPath string) string {
	if resolvedPath != "" {
		return filepath.Base(resolvedPath)
	}
	if argv0 == "" {
		return ""
	}
	return filepath.Base(argv0)
}

func commandFull(argv0 string, resolvedPath string) string {
	if resolvedPath != "" {
		return resolvedPath
	}
	return argv0
}
