package ptywrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// Options controls PTY execution behavior.
type Options struct {
	RawMode bool
	Output  io.Writer
}

// RunCommand starts cmd under a PTY and proxies IO.
func RunCommand(ctx context.Context, cmd *exec.Cmd, opts Options) (int, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("start pty: %w", err)
	}
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	restore, err := maybeMakeRaw(opts.RawMode)
	if err != nil {
		return 1, err
	}
	if restore != nil {
		defer restore()
	}

	_ = pty.InheritSize(os.Stdin, ptmx)
	stopSignals := forwardSignals(cmd.Process, ptmx)
	defer stopSignals()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	go copyWithContext(ctx, out, ptmx, errCh)

	waitErr := cmd.Wait()
	cancel()
	_ = ptmx.Close()
	_ = closeOutput(out)
	<-errCh

	if waitErr == nil {
		return 0, nil
	}
	return exitCode(waitErr), nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader, errCh chan<- error) {
	_, err := io.Copy(dst, src)
	select {
	case errCh <- err:
	case <-ctx.Done():
	}
}

func closeOutput(out io.Writer) error {
	if closer, ok := out.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func maybeMakeRaw(enable bool) (func(), error) {
	if !enable {
		return nil, nil
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("set raw mode: %w", err)
	}
	return func() { _ = term.Restore(fd, state) }, nil
}

func forwardSignals(proc *os.Process, ptmx *os.File) func() {
	if proc == nil {
		return func() {}
	}
	ch := make(chan os.Signal, 8)
	signal.Notify(ch, syscall.SIGWINCH, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for sig := range ch {
			switch sig {
			case syscall.SIGWINCH:
				// Best-effort resize; ignore errors.
				_ = pty.InheritSize(os.Stdin, ptmx)
			default:
				_ = proc.Signal(sig)
			}
		}
	}()

	return func() {
		signal.Stop(ch)
		close(ch)
		<-done
	}
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return 128 + int(status.Signal())
			}
			return status.ExitStatus()
		}
	}
	return 1
}
