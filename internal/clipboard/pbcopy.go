package clipboard

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

var execCommand = exec.Command

// CopyBytes copies data to the macOS clipboard using pbcopy.
func CopyBytes(data []byte) error {
	cmd := execCommand("pbcopy")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("pbcopy stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pbcopy start: %w", err)
	}
	if _, err := io.Copy(stdin, bytes.NewReader(data)); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("pbcopy write: %w", err)
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pbcopy wait: %w", err)
	}
	return nil
}
