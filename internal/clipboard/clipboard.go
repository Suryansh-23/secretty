package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Backend identifies a clipboard command backend.
type Backend string

const (
	BackendAuto   Backend = "auto"
	BackendPbcopy Backend = "pbcopy"
	BackendWlCopy Backend = "wl-copy"
	BackendXclip  Backend = "xclip"
	BackendXsel   Backend = "xsel"
	BackendNone   Backend = "none"
)

var lookPath = exec.LookPath

// CopyBytes writes data to the clipboard using the requested backend.
func CopyBytes(backend string, data []byte) error {
	resolved, err := ResolveBackend(backend)
	if err != nil {
		return err
	}
	if resolved == BackendNone {
		return errors.New("clipboard disabled")
	}
	return copyBytes(resolved, data)
}

// VerifyBytes checks whether the clipboard matches the expected payload.
func VerifyBytes(backend string, expected []byte) error {
	resolved, err := ResolveBackend(backend)
	if err != nil {
		return err
	}
	if resolved == BackendNone {
		return errors.New("clipboard disabled")
	}
	actual, err := pasteBytes(resolved)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("clipboard verification failed")
	}
	return nil
}

// ResolveBackend converts a backend string into a concrete backend.
func ResolveBackend(backend string) (Backend, error) {
	requested := Backend(strings.ToLower(strings.TrimSpace(backend)))
	if requested == "" {
		requested = BackendAuto
	}
	switch requested {
	case BackendAuto:
		return autoBackend()
	case BackendPbcopy, BackendWlCopy, BackendXclip, BackendXsel, BackendNone:
		return requested, nil
	default:
		return "", fmt.Errorf("unsupported clipboard backend: %q", backend)
	}
}

func autoBackend() (Backend, error) {
	candidates := autoBackendCandidates()
	if len(candidates) == 0 {
		return "", errors.New("no clipboard backend available (missing display server)")
	}
	for _, candidate := range candidates {
		if hasCommand(candidate) {
			return candidate, nil
		}
	}
	var names []string
	for _, candidate := range candidates {
		names = append(names, string(candidate))
	}
	return "", fmt.Errorf("no clipboard backend found; install one of: %s", strings.Join(names, ", "))
}

func hasCommand(backend Backend) bool {
	if backend == BackendNone || backend == "" {
		return false
	}
	_, err := lookPath(string(backend))
	return err == nil
}
