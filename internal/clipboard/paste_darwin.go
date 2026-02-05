//go:build darwin
// +build darwin

package clipboard

import "fmt"

func pasteBytes(backend Backend) ([]byte, error) {
	switch backend {
	case BackendPbcopy:
		return runPasteCommand("pbpaste", nil)
	default:
		return nil, fmt.Errorf("clipboard backend %q is not supported on darwin", backend)
	}
}
