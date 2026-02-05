package clipboard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

var execCommand = exec.CommandContext

const copyTimeout = 2 * time.Second

func runCopyCommand(command string, args []string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), copyTimeout)
	defer cancel()

	cmd := execCommand(ctx, command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("%s stdin: %w", command, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s start: %w", command, err)
	}
	if _, err := io.Copy(stdin, bytes.NewReader(data)); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("%s write: %w", command, err)
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s timeout: %w", command, ctx.Err())
		}
		return fmt.Errorf("%s wait: %w", command, err)
	}
	return nil
}
