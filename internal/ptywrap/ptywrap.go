package ptywrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/suryansh-23/secretty/internal/debug"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Options controls PTY execution behavior.
type Options struct {
	RawMode bool
	Output  io.Writer
	Logger  *debug.Logger
}

// RunCommand starts cmd under a PTY and proxies IO.
func RunCommand(ctx context.Context, cmd *exec.Cmd, opts Options) (int, error) {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	stdinFD := int(os.Stdin.Fd())
	isTTY := term.IsTerminal(stdinFD)
	if opts.Logger != nil {
		opts.Logger.Infof("ptywrap: stdin_is_tty=%t", isTTY)
	}
	ensureTermFallback(cmd, opts.Logger)

	ptmx, err := startWithPTY(cmd, isTTY, opts.Logger)
	if err != nil {
		return 1, err
	}
	defer func() { _ = ptmx.Close() }()

	restore, err := maybeMakeRaw(opts.RawMode && isTTY)
	if err != nil {
		return 1, err
	}
	if restore != nil {
		defer restore()
	}

	if isTTY {
		_ = pty.InheritSize(os.Stdin, ptmx)
		setForegroundProcessGroup(ptmx, cmd.Process, opts.Logger)
	}
	stopSignals := forwardSignals(cmd.Process, ptmx, isTTY)
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

func startWithPTY(cmd *exec.Cmd, isTTY bool, logger *debug.Logger) (*os.File, error) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	defer func() {
		_ = tty.Close()
	}()

	if isTTY {
		if err := inheritTermios(tty); err != nil {
			_ = ptmx.Close()
			return nil, err
		}
	}
	if winsize := hostWinsize(int(os.Stdin.Fd()), logger); winsize != nil {
		if err := pty.Setsize(ptmx, winsize); err != nil {
			_ = ptmx.Close()
			return nil, fmt.Errorf("set pty size: %w", err)
		}
	}

	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = isTTY
	cmd.SysProcAttr.Ctty = 0
	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		return nil, fmt.Errorf("start pty command: %w", err)
	}
	return ptmx, nil
}

func hostWinsize(fd int, logger *debug.Logger) *pty.Winsize {
	if fd < 0 || !term.IsTerminal(fd) {
		return nil
	}
	cols, rows, err := term.GetSize(fd)
	if err != nil || cols <= 0 || rows <= 0 {
		if logger != nil {
			logger.Infof("ptywrap: winsize_unavailable=%v", err)
		}
		return nil
	}
	if logger != nil {
		logger.Infof("ptywrap: winsize=%dx%d", cols, rows)
	}
	return &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
}

func ensureTermFallback(cmd *exec.Cmd, logger *debug.Logger) {
	if cmd == nil {
		return
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	if logger != nil {
		term := envValue(cmd.Env, "TERM")
		logger.Infof("ptywrap: term=%s", term)
	}
	if override := envValue(cmd.Env, "SECRETTY_TERM"); override != "" {
		cmd.Env = setEnv(cmd.Env, "TERM", override)
		if logger != nil {
			logger.Infof("ptywrap: term_override=%s", override)
		}
		return
	}
	term := envValue(cmd.Env, "TERM")
	if term == "" || terminfoExists(term, cmd.Env) {
		return
	}
	fallback := "xterm-256color"
	if term == fallback {
		return
	}
	cmd.Env = setEnv(cmd.Env, "TERM", fallback)
	if logger != nil {
		logger.Infof("ptywrap: term_fallback=%s", fallback)
	}
}

func terminfoExists(term string, env []string) bool {
	if term == "" {
		return false
	}
	first := string(term[0])
	for _, dir := range terminfoDirs(env) {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, first, term)); err == nil {
			return true
		}
	}
	return false
}

func terminfoDirs(env []string) []string {
	var dirs []string
	if terminfo := envValue(env, "TERMINFO"); terminfo != "" {
		dirs = append(dirs, terminfo)
	}
	if terminfoDirs := envValue(env, "TERMINFO_DIRS"); terminfoDirs != "" {
		parts := strings.Split(terminfoDirs, ":")
		for _, part := range parts {
			if part == "" {
				dirs = append(dirs, "/usr/share/terminfo")
				continue
			}
			dirs = append(dirs, part)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".terminfo"))
	}
	dirs = append(dirs,
		"/usr/share/terminfo",
		"/usr/local/share/terminfo",
		"/opt/homebrew/share/terminfo",
	)
	return dirs
}

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		entry := env[i]
		if strings.HasPrefix(entry, key+"=") {
			return entry[len(key)+1:]
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	if env == nil {
		return []string{key + "=" + value}
	}
	for i, entry := range env {
		if strings.HasPrefix(entry, key+"=") {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
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

func forwardSignals(proc *os.Process, ptmx *os.File, resize bool) func() {
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
				if resize {
					// Best-effort resize; ignore errors.
					_ = pty.InheritSize(os.Stdin, ptmx)
				}
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

func setForegroundProcessGroup(ptmx *os.File, proc *os.Process, logger *debug.Logger) {
	if ptmx == nil || proc == nil {
		return
	}
	pgid, err := syscall.Getpgid(proc.Pid)
	if err != nil {
		if logger != nil {
			logger.Infof("ptywrap: getpgid_failed=%v", err)
		}
		return
	}
	if err := unix.IoctlSetInt(int(ptmx.Fd()), unix.TIOCSPGRP, pgid); err != nil {
		if logger != nil {
			logger.Infof("ptywrap: set_fg_pgrp_failed=%v", err)
		}
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
