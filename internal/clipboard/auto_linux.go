//go:build linux
// +build linux

package clipboard

import (
	"os"
	"strings"
)

func autoBackendCandidates() []Backend {
	var candidates []Backend
	if isWayland() {
		candidates = append(candidates, BackendWlCopy)
	}
	if isX11() {
		candidates = append(candidates, BackendXclip, BackendXsel)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

func isWayland() bool {
	if v := strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")); strings.EqualFold(v, "wayland") {
		return true
	}
	return strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func isX11() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != ""
}
