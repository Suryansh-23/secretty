package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/ptywrap"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
)

type appState struct {
	cfg      config.Config
	cfgFound bool
}

type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.code)
}

func main() {
	state := &appState{}
	rootCmd := newRootCmd(state)
	if err := rootCmd.Execute(); err != nil {
		if exitErr, ok := err.(*exitCodeError); ok {
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd(state *appState) *cobra.Command {
	var (
		cfgPath     string
		strictFlag  bool
		debugFlag   bool
		noInitHints bool
	)

	rootCmd := &cobra.Command{
		Use:          "secretty",
		Short:        "Protect terminal output by redacting secrets",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, found, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			applyOverrides(&cfg, strictFlag, debugFlag)
			state.cfg = cfg
			state.cfgFound = found
			if !found && !noInitHints && cmd.Name() != "init" {
				fmt.Fprintln(os.Stderr, "secretty: no config found; run `secretty init`")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			command := defaultShellCommand()
			return runWithPTY(cmd.Context(), state.cfg, command)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&strictFlag, "strict", false, "enable strict mode (no reveal to screen)")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable sanitized debug logging")
	rootCmd.PersistentFlags().BoolVar(&noInitHints, "no-init-hints", false, "suppress init guidance")

	rootCmd.AddCommand(newShellCmd(state))
	rootCmd.AddCommand(newRunCmd(state))
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newCopyCmd())
	rootCmd.AddCommand(newDoctorCmd())

	return rootCmd
}

func applyOverrides(cfg *config.Config, strictFlag, debugFlag bool) {
	if strictFlag {
		cfg.Mode = types.ModeStrict
		cfg.Strict.NoReveal = true
	}
	if debugFlag {
		cfg.Debug.Enabled = true
	}
}

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
				command = exec.Command(shellArgs[0], shellArgs[1:]...)
			}
			return runWithPTY(cmd.Context(), state.cfg, command)
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
			return runWithPTY(cmd.Context(), state.cfg, command)
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Run the first-time setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("init command not implemented yet")
		},
	}
}

func newCopyCmd() *cobra.Command {
	copyCmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy redacted secrets without rendering",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	copyCmd.AddCommand(&cobra.Command{
		Use:   "last",
		Short: "Copy the last redacted secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("copy last not implemented yet")
		},
	})
	return copyCmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Print environment diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("doctor command not implemented yet")
		},
	}
}

func defaultShellCommand() *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	return exec.Command(shell, "-l")
}

func runWithPTY(ctx context.Context, cfg config.Config, command *exec.Cmd) error {
	command.Env = os.Environ()
	detector := detect.NewEngine(cfg)
	stream := redact.NewStream(os.Stdout, cfg, detector)
	exitCode, err := ptywrap.RunCommand(ctx, command, ptywrap.Options{RawMode: true, Output: stream})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return &exitCodeError{code: exitCode}
	}
	return nil
}
