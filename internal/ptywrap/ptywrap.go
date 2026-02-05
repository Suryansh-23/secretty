package ptywrap

import (
	"bufio"
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

	var termios *unix.Termios
	if isTTY {
		if captured, err := getTermios(stdinFD); err == nil {
			termios = captured
		}
	}
	restore, err := maybeMakeRaw(opts.RawMode && isTTY)
	if err != nil {
		return 1, err
	}
	if restore != nil {
		defer restore()
	}

	ptmx, err := startWithPTY(cmd, isTTY, termios, opts.Logger)
	if err != nil {
		return 1, err
	}
	defer func() { _ = ptmx.Close() }()

	if isTTY {
		if err := pty.InheritSize(os.Stdin, ptmx); err != nil && opts.Logger != nil {
			opts.Logger.Infof("ptywrap: inherit_size_failed=%v", err)
		}
	}
	stopSignals := forwardSignals(cmd.Process, ptmx, isTTY, opts.Logger)
	defer stopSignals()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go copyInput(ctx, ptmx, os.Stdin, opts.Logger)
	go copyWithContext(ctx, out, ptmx, errCh)

	waitErr := cmd.Wait()
	cancel()
	_ = ptmx.Close()
	if err := closeOutput(out); err != nil && opts.Logger != nil {
		opts.Logger.Infof("ptywrap: close_output_failed=%v", err)
	}
	<-errCh

	if waitErr == nil {
		return 0, nil
	}
	return exitCode(waitErr), nil
}

func startWithPTY(cmd *exec.Cmd, isTTY bool, termios *unix.Termios, logger *debug.Logger) (*os.File, error) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	defer func() {
		_ = tty.Close()
	}()

	if isTTY && termios != nil {
		if err := setTermios(int(tty.Fd()), termios); err != nil {
			_ = ptmx.Close()
			return nil, fmt.Errorf("set pty terminal settings: %w", err)
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
	if isTTY {
		setForegroundProcessGroup(tty, cmd.Process, logger)
		flushPendingInput(tty, logger)
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
		"/lib/terminfo",
		"/usr/lib/terminfo",
		"/etc/terminfo",
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

func copyInput(ctx context.Context, dst *os.File, src io.Reader, logger *debug.Logger) {
	reader := bufio.NewReader(src)
	filter := newResponseFilter(responseDrainWindow)
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if filter.active() {
				filtered := filter.Filter(chunk)
				if !filter.active() {
					filtered = append(filtered, filter.Flush()...)
				}
				if len(filtered) > 0 {
					if _, err := dst.Write(filtered); err != nil {
						if logger != nil {
							logger.Infof("ptywrap: stdin_write_error=%v", err)
						}
						return
					}
				}
			} else {
				if pending := filter.Flush(); len(pending) > 0 {
					if _, err := dst.Write(pending); err != nil {
						if logger != nil {
							logger.Infof("ptywrap: stdin_write_error=%v", err)
						}
						return
					}
				}
				if _, err := dst.Write(chunk); err != nil {
					if logger != nil {
						logger.Infof("ptywrap: stdin_write_error=%v", err)
					}
					return
				}
			}
		}
		if err != nil {
			if pending := filter.Flush(); len(pending) > 0 {
				if _, writeErr := dst.Write(pending); writeErr != nil {
					if logger != nil {
						logger.Infof("ptywrap: stdin_write_error=%v", writeErr)
					}
					return
				}
			}
			if logger != nil && !errors.Is(err, io.EOF) {
				logger.Infof("ptywrap: stdin_copy_error=%v", err)
			}
			return
		}
	}
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
		if restoreErr := term.Restore(fd, state); restoreErr != nil {
			return nil, fmt.Errorf("restore terminal: %w", restoreErr)
		}
		return nil, err
	}
	if termios != nil {
		// Re-enable signals so Ctrl+C/Ctrl+Z still generate SIGINT/SIGTSTP.
		termios.Lflag |= unix.ISIG
		if err := setTermios(fd, termios); err != nil {
			if restoreErr := term.Restore(fd, state); restoreErr != nil {
				return nil, fmt.Errorf("restore terminal: %w", restoreErr)
			}
			return nil, fmt.Errorf("set termios: %w", err)
		}
	}
	return func() {
		if err := term.Restore(fd, state); err != nil {
			_ = err
		}
	}, nil
}

func forwardSignals(proc *os.Process, ptmx *os.File, resize bool, logger *debug.Logger) func() {
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
					if err := pty.InheritSize(os.Stdin, ptmx); err != nil && logger != nil {
						logger.Infof("ptywrap: resize_failed=%v", err)
					}
				}
			case syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP:
				if ptmx != nil {
					if b, ok := controlByteForSignal(sig); ok {
						if _, err := ptmx.Write([]byte{b}); err != nil && logger != nil {
							logger.Infof("ptywrap: signal_write_failed=%v", err)
						}
						continue
					}
				}
				if err := proc.Signal(sig); err != nil && logger != nil {
					logger.Infof("ptywrap: signal_forward_failed=%v", err)
				}
			default:
				if err := proc.Signal(sig); err != nil && logger != nil {
					logger.Infof("ptywrap: signal_forward_failed=%v", err)
				}
			}
		}
	}()

	return func() {
		signal.Stop(ch)
		close(ch)
		<-done
	}
}

func controlByteForSignal(sig os.Signal) (byte, bool) {
	switch sig {
	case syscall.SIGINT:
		return 0x03, true
	case syscall.SIGQUIT:
		return 0x1c, true
	case syscall.SIGTSTP:
		return 0x1a, true
	default:
		return 0, false
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
