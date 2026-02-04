//go:build darwin
// +build darwin

package ptywrap

import (
	"errors"
	"os"
	"syscall"

	"github.com/suryansh-23/secretty/internal/debug"
	"golang.org/x/sys/unix"
)

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

func flushPendingInput(tty *os.File, logger *debug.Logger) {
	if tty == nil {
		return
	}
	if err := unix.IoctlSetInt(int(tty.Fd()), syscall.TIOCFLUSH, syscall.TCIFLUSH); err != nil {
		if errors.Is(err, unix.ENOTTY) || errors.Is(err, syscall.ENOTTY) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTSUP) {
			return
		}
		if logger != nil {
			logger.Infof("ptywrap: tcflush_failed=%v", err)
		}
		return
	}
	if logger != nil {
		logger.Infof("ptywrap: tcflush=ok")
	}
}
