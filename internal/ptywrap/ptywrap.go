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
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Options controls PTY execution behavior.
type Options struct {
	RawMode bool
	Output  io.Writer
}

// RunCommand starts cmd under a PTY and proxies IO.
func RunCommand(ctx context.Context, cmd *exec.Cmd, opts Options) (int, error) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		return 1, fmt.Errorf("open pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	if err := inheritTermios(tty); err != nil {
		_ = tty.Close()
		return 1, err
	}
	prepareCommand(cmd, tty)
	if err := cmd.Start(); err != nil {
		_ = tty.Close()
		return 1, fmt.Errorf("start pty command: %w", err)
	}
	_ = tty.Close()

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
	return makeRawWithSignals(fd)
}

func makeRawWithSignals(fd int) (func(), error) {
	if fd < 0 || !term.IsTerminal(fd) {
		return nil, nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("make raw: %w", err)
	}
	termios, err := getTermios(fd)
	if err != nil {
		_ = term.Restore(fd, state)
		return nil, err
	}
	if termios != nil {
		// Re-enable signals so Ctrl+C/Ctrl+Z still generate SIGINT/SIGTSTP.
		termios.Lflag |= unix.ISIG
		if err := setTermios(fd, termios); err != nil {
			_ = term.Restore(fd, state)
			return nil, fmt.Errorf("set termios: %w", err)
		}
	}
	return func() { _ = term.Restore(fd, state) }, nil
}

func getTermios(fd int) (*unix.Termios, error) {
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		if errors.Is(err, unix.ENOTTY) || errors.Is(err, syscall.ENOTTY) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTSUP) {
			return nil, nil
		}
		return nil, err
	}
	copy := *termios
	return &copy, nil
}

func setTermios(fd int, termios *unix.Termios) error {
	if termios == nil {
		return nil
	}
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, termios)
}

func inheritTermios(tty *os.File) error {
	if tty == nil {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	termios, err := getTermios(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("get terminal settings: %w", err)
	}
	if termios == nil {
		return nil
	}
	if err := setTermios(int(tty.Fd()), termios); err != nil {
		return fmt.Errorf("set pty terminal settings: %w", err)
	}
	return nil
}

func prepareCommand(cmd *exec.Cmd, tty *os.File) {
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Ctty = 0
}

func forwardSignals(proc *os.Process, ptmx *os.File) func() {
	if proc == nil {
		return func() {}
	}
	ch := make(chan os.Signal, 8)
	signal.Notify(ch, syscall.SIGWINCH, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGTSTP)

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
