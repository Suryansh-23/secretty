//go:build darwin
// +build darwin

package clipboard

import "fmt"

func copyBytes(backend Backend, data []byte) error {
	switch backend {
	case BackendPbcopy:
		return runCopyCommand("pbcopy", nil, data)
	default:
		return fmt.Errorf("clipboard backend %q is not supported on darwin", backend)
	}
}
