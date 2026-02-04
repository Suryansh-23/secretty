package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newShellCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "shell -- <shell>",
		Short: "Start a protected interactive shell",
		RunE: func(cmd *cobra.Command, args []string) error {
			var command *exec.Cmd
			if cmd.ArgsLenAtDash() == -1 {
				command = defaultShellCommand()
			} else {
				shellArgs := cmd.Flags().Args()
				if len(shellArgs) == 0 {
					return errors.New("shell requires a command after --")
				}
				var err error
				command, err = shellCommandFromArgs(shellArgs)
				if err != nil {
					return err
				}
			}
			return runWithPTY(cmd.Context(), state.cfg, state.cfgPath, command, state.cache, state.logger, true)
		},
	}
}

func newRunCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "run -- <cmd...>",
		Short: "Run a command under a protected PTY",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.ArgsLenAtDash() == -1 {
				return errors.New("run requires -- before the command")
			}
			runArgs := cmd.Flags().Args()
			if len(runArgs) == 0 {
				return errors.New("run requires a command after --")
			}
			command := exec.Command(runArgs[0], runArgs[1:]...)
			return runWithPTY(cmd.Context(), state.cfg, state.cfgPath, command, state.cache, state.logger, false)
		},
	}
}

func defaultShellCommand() *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	return exec.Command(shell, "-l", "-i")
}

func shellCommandFromArgs(args []string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, errors.New("shell requires a command after --")
	}
	if len(args) == 1 && looksLikeShell(args[0]) {
		return exec.Command(args[0], "-l", "-i"), nil
	}
	return exec.Command(args[0], args[1:]...), nil
}

func looksLikeShell(path string) bool {
	base := filepath.Base(path)
	switch base {
	case "zsh", "bash", "fish", "sh":
		return true
	default:
		return false
	}
}
