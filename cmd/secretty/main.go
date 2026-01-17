package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/clipboard"
	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/ptywrap"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
)

type appState struct {
	cfg      config.Config
	cfgFound bool
	cache    *cache.Cache
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
			state.cache = ensureCache(state.cache, cfg)
			if !found && !noInitHints && cmd.Name() != "init" {
				fmt.Fprintln(os.Stderr, "secretty: no config found; run `secretty init`")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			command := defaultShellCommand()
			return runWithPTY(cmd.Context(), state.cfg, command, state.cache)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&strictFlag, "strict", false, "enable strict mode (no reveal to screen)")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable sanitized debug logging")
	rootCmd.PersistentFlags().BoolVar(&noInitHints, "no-init-hints", false, "suppress init guidance")

	rootCmd.AddCommand(newShellCmd(state))
	rootCmd.AddCommand(newRunCmd(state))
	rootCmd.AddCommand(newInitCmd(&cfgPath))
	rootCmd.AddCommand(newCopyCmd(state))
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
			return runWithPTY(cmd.Context(), state.cfg, command, state.cache)
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
			return runWithPTY(cmd.Context(), state.cfg, command, state.cache)
		},
	}
}

func newInitCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Run the first-time setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(*cfgPath)
			if err != nil {
				return err
			}
			if exists(path) {
				confirm := false
				form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Config exists. Overwrite?").Value(&confirm)))
				if err := form.Run(); err != nil {
					return err
				}
				if !confirm {
					return errors.New("init cancelled")
				}
			}

			printEnvSummary()
			cfg := config.DefaultConfig()

			mode := string(cfg.Mode)
			enableWeb3 := cfg.Rulesets.Web3.Enabled
			copyEnabled := cfg.Overrides.CopyWithoutRender.Enabled
			requireConfirm := cfg.Overrides.CopyWithoutRender.RequireConfirm
			ttl := cfg.Overrides.CopyWithoutRender.TTLSeconds

			modeForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().Title("Choose mode").Value(&mode).Options(
						huh.NewOption("Demo (default)", string(types.ModeDemo)),
						huh.NewOption("Strict recording", string(types.ModeStrict)),
						huh.NewOption("Warn-only", string(types.ModeWarn)),
					),
				),
			)
			if err := modeForm.Run(); err != nil {
				return err
			}

			web3Form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().Title("Enable Web3 ruleset (EVM keys)?").Value(&enableWeb3),
				),
			)
			if err := web3Form.Run(); err != nil {
				return err
			}

			copyForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().Title("Enable copy-without-render?").Value(&copyEnabled),
				),
			)
			if err := copyForm.Run(); err != nil {
				return err
			}

			if copyEnabled {
				reqConfirmForm := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().Title("Require confirmation before copying?").Value(&requireConfirm),
					),
				)
				if err := reqConfirmForm.Run(); err != nil {
					return err
				}

				ttlStr := strconv.Itoa(ttl)
				ttlForm := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().Title("Copy TTL seconds").Value(&ttlStr).Validate(func(v string) error {
							value, err := strconv.Atoi(strings.TrimSpace(v))
							if err != nil || value < 0 {
								return errors.New("enter a non-negative integer")
							}
							return nil
						}),
					),
				)
				if err := ttlForm.Run(); err != nil {
					return err
				}
				parsedTTL, _ := strconv.Atoi(strings.TrimSpace(ttlStr))
				ttl = parsedTTL
			}

			cfg.Mode = types.Mode(mode)
			cfg.Rulesets.Web3.Enabled = enableWeb3
			cfg.Overrides.CopyWithoutRender.Enabled = copyEnabled
			cfg.Overrides.CopyWithoutRender.RequireConfirm = requireConfirm
			cfg.Overrides.CopyWithoutRender.TTLSeconds = ttl
			if cfg.Mode == types.ModeStrict {
				cfg.Strict.NoReveal = true
			}

			if err := runSelfTest(cfg); err != nil {
				return err
			}

			fmt.Println("Suggested alias: alias safe=secretty")

			if err := config.Write(path, cfg); err != nil {
				return err
			}
			fmt.Printf("Wrote config to %s\n", path)
			return nil
		},
	}
}

func newCopyCmd(state *appState) *cobra.Command {
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
			if !state.cfg.Overrides.CopyWithoutRender.Enabled {
				return errors.New("copy-without-render is disabled")
			}
			if state.cfg.Mode == types.ModeStrict && state.cfg.Strict.DisableCopyOriginal {
				return errors.New("copy original is disabled in strict mode")
			}
			if state.cache == nil {
				return errors.New("no secret cache available")
			}
			if state.cfg.Overrides.CopyWithoutRender.RequireConfirm {
				confirm := false
				form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Copy last secret to clipboard?").Value(&confirm)))
				if err := form.Run(); err != nil {
					return err
				}
				if !confirm {
					return errors.New("copy cancelled")
				}
			}
			record, ok := state.cache.GetLast()
			if !ok {
				return errors.New("no secrets cached")
			}
			if err := clipboard.CopyBytes(record.Original); err != nil {
				return err
			}
			if state.cfg.Redaction.IncludeEventID {
				fmt.Printf("Copied secret %d to clipboard\n", record.ID)
			} else {
				fmt.Println("Copied secret to clipboard")
			}
			return nil
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

func runWithPTY(ctx context.Context, cfg config.Config, command *exec.Cmd, cache *cache.Cache) error {
	command.Env = os.Environ()
	detector := detect.NewEngine(cfg)
	stream := redact.NewStream(os.Stdout, cfg, detector, cache)
	exitCode, err := ptywrap.RunCommand(ctx, command, ptywrap.Options{RawMode: true, Output: stream})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return &exitCodeError{code: exitCode}
	}
	return nil
}

func ensureCache(existing *cache.Cache, cfg config.Config) *cache.Cache {
	if !cfg.Overrides.CopyWithoutRender.Enabled {
		return existing
	}
	ttl := time.Duration(cfg.Overrides.CopyWithoutRender.TTLSeconds) * time.Second
	if existing == nil {
		return cache.New(64, ttl)
	}
	existing.SetTTL(ttl)
	return existing
}

func resolveConfigPath(override string) (string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		return override, nil
	}
	return config.DefaultPath()
}

func exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func printEnvSummary() {
	shell := os.Getenv("SHELL")
	termName := os.Getenv("TERM")
	inTmux := os.Getenv("TMUX") != ""
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		cols = 0
		rows = 0
	}
	fmt.Printf("Detected shell=%s TERM=%s tmux=%t size=%dx%d\n", shell, termName, inTmux, cols, rows)
}

func runSelfTest(cfg config.Config) error {
	key, err := config.SyntheticEvmKey()
	if err != nil {
		return err
	}
	line := []byte("PRIVATE_KEY=" + key)
	detector := detect.NewEngine(cfg)
	matches := detector.Find(line)
	redactor := redact.NewRedactor(cfg)
	out, err := redactor.Apply(line, matches)
	if err != nil {
		return err
	}
	if strings.Contains(string(out), key) {
		return errors.New("self-test failed: secret was not redacted")
	}
	fmt.Printf("Self-test output: %s\n", string(out))
	return nil
}
