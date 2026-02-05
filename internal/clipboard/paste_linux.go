//go:build !darwin
// +build !darwin

package clipboard

import "fmt"

func pasteBytes(backend Backend) ([]byte, error) {
	switch backend {
	case BackendWlCopy:
		return runPasteCommand("wl-paste", []string{"--no-newline"})
	case BackendXclip:
		return runPasteCommand("xclip", []string{"-o", "-selection", "clipboard"})
	case BackendXsel:
		return runPasteCommand("xsel", []string{"--clipboard", "--output"})
	default:
		return nil, fmt.Errorf("clipboard backend %q is not supported for paste", backend)
	}
}
