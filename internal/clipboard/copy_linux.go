//go:build linux
// +build linux

package clipboard

import "fmt"

func copyBytes(backend Backend, data []byte) error {
	switch backend {
	case BackendWlCopy:
		return runCopyCommand("wl-copy", nil, data)
	case BackendXclip:
		return runCopyCommand("xclip", []string{"-selection", "clipboard"}, data)
	case BackendXsel:
		return runCopyCommand("xsel", []string{"--clipboard", "--input"}, data)
	default:
		return fmt.Errorf("clipboard backend %q is not supported on linux", backend)
	}
}
