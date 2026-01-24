package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/clipboard"
	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/debug"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/ipc"
	"github.com/suryansh-23/secretty/internal/ptywrap"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
	"github.com/suryansh-23/secretty/internal/ui"
)

type appState struct {
	cfg      config.Config
	cfgFound bool
	cache    *cache.Cache
	logger   *debug.Logger
	cfgPath  string
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
			resolvedPath, err := resolveConfigPath(cfgPath)
			if err != nil {
				return err
			}
			cfg, found, err := config.Load(resolvedPath)
			if err != nil {
				return err
			}
			applyOverrides(&cfg, strictFlag, debugFlag)
			state.cfg = cfg
			state.cfgFound = found
			state.cache = ensureCache(state.cache, cfg)
			state.logger = debug.New(cfg.Debug.Enabled)
			state.cfgPath = resolvedPath
			if !found && !noInitHints && cmd.Name() != "init" {
				fmt.Fprintln(os.Stderr, "secretty: no config found; run `secretty init`")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			command := defaultShellCommand()
			return runWithPTY(cmd.Context(), state.cfg, command, state.cache, state.logger, true)
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
	rootCmd.AddCommand(newDoctorCmd(state))

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
				var err error
				command, err = shellCommandFromArgs(shellArgs)
				if err != nil {
					return err
				}
			}
			return runWithPTY(cmd.Context(), state.cfg, command, state.cache, state.logger, true)
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
			return runWithPTY(cmd.Context(), state.cfg, command, state.cache, state.logger, false)
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

			cfg := config.DefaultConfig()
			mode := string(cfg.Mode)
			maskStyle := string(cfg.Masking.Style)
			selectedRulesets := defaultRulesetSelections(cfg)
			copyEnabled := cfg.Overrides.CopyWithoutRender.Enabled
			requireConfirm := cfg.Overrides.CopyWithoutRender.RequireConfirm
			ttlStr := strconv.Itoa(cfg.Overrides.CopyWithoutRender.TTLSeconds)
			overwrite := false

			envNote := huh.NewNote().
				Title("Environment").
				Description(envSummary()).
				Next(true)

			form := huh.NewForm(
				huh.NewGroup(envNote),
				huh.NewGroup(
					huh.NewConfirm().Title("Config exists. Overwrite?").Value(&overwrite),
				).WithHideFunc(func() bool { return !exists(path) }),
				huh.NewGroup(
					huh.NewSelect[string]().Title("Choose mode").Value(&mode).Options(
						huh.NewOption("Demo (default)", string(types.ModeDemo)),
						huh.NewOption("Strict recording", string(types.ModeStrict)),
						huh.NewOption("Warn-only", string(types.ModeWarn)),
					),
				),
				huh.NewGroup(
					huh.NewSelect[string]().Title("Redaction style").Value(&maskStyle).Options(
						huh.NewOption("Classic blocks", string(types.MaskStyleBlock)),
						huh.NewOption("Glow blocks", string(types.MaskStyleGlow)),
						huh.NewOption("Morse code", string(types.MaskStyleMorse)),
					),
				),
				huh.NewGroup(
					huh.NewMultiSelect[string]().Title("Enable rulesets").Value(&selectedRulesets).Options(
						huh.NewOption("Web3 (EVM keys)", "web3"),
						huh.NewOption("API keys", "api_keys"),
						huh.NewOption("Auth tokens (JWT/Bearer)", "auth_tokens"),
						huh.NewOption("Cloud credentials", "cloud"),
						huh.NewOption("Passwords", "passwords"),
					),
				),
				huh.NewGroup(
					huh.NewConfirm().Title("Enable copy-without-render?").Value(&copyEnabled),
				),
				huh.NewGroup(
					huh.NewConfirm().Title("Require confirmation before copying?").Value(&requireConfirm),
				).WithHideFunc(func() bool { return !copyEnabled }),
				huh.NewGroup(
					huh.NewInput().Title("Copy TTL seconds").Value(&ttlStr).Validate(func(v string) error {
						value, err := strconv.Atoi(strings.TrimSpace(v))
						if err != nil || value < 0 {
							return errors.New("enter a non-negative integer")
						}
						return nil
					}),
				).WithHideFunc(func() bool { return !copyEnabled }),
			).WithTheme(ui.Theme())

			if err := runAnimatedForm(form); err != nil {
				return err
			}
			if exists(path) && !overwrite {
				return errors.New("init cancelled")
			}

			cfg.Mode = types.Mode(mode)
			cfg.Masking.Style = types.MaskStyle(maskStyle)
			applyRulesetSelections(&cfg, selectedRulesets)
			cfg.Overrides.CopyWithoutRender.Enabled = copyEnabled
			cfg.Overrides.CopyWithoutRender.RequireConfirm = requireConfirm
			if copyEnabled {
				parsedTTL, _ := strconv.Atoi(strings.TrimSpace(ttlStr))
				cfg.Overrides.CopyWithoutRender.TTLSeconds = parsedTTL
			}
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
			if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
				resp, err := ipc.CopyLast(socketPath)
				if err != nil {
					return err
				}
				if state.cfg.Redaction.IncludeEventID && resp.ID > 0 {
					fmt.Printf("Copied secret %d to clipboard\n", resp.ID)
				} else {
					fmt.Println("Copied secret to clipboard")
				}
				return nil
			}
			if state.cache == nil {
				return errors.New("no secret cache available")
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

func newDoctorCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Print environment diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(state)
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

func defaultRulesetSelections(cfg config.Config) []string {
	var out []string
	if cfg.Rulesets.Web3.Enabled {
		out = append(out, "web3")
	}
	if cfg.Rulesets.APIKeys.Enabled {
		out = append(out, "api_keys")
	}
	if cfg.Rulesets.AuthTokens.Enabled {
		out = append(out, "auth_tokens")
	}
	if cfg.Rulesets.Cloud.Enabled {
		out = append(out, "cloud")
	}
	if cfg.Rulesets.Passwords.Enabled {
		out = append(out, "passwords")
	}
	return out
}

func applyRulesetSelections(cfg *config.Config, selected []string) {
	set := make(map[string]bool, len(selected))
	for _, name := range selected {
		set[name] = true
	}
	cfg.Rulesets.Web3.Enabled = set["web3"]
	cfg.Rulesets.APIKeys.Enabled = set["api_keys"]
	cfg.Rulesets.AuthTokens.Enabled = set["auth_tokens"]
	cfg.Rulesets.Cloud.Enabled = set["cloud"]
	cfg.Rulesets.Passwords.Enabled = set["passwords"]
}

func startIPCServer(cfg config.Config, cache *cache.Cache) (string, func(), error) {
	if cache == nil {
		return "", nil, nil
	}
	if !cfg.Overrides.CopyWithoutRender.Enabled {
		return "", nil, nil
	}
	if cfg.Mode == types.ModeStrict && cfg.Strict.DisableCopyOriginal {
		return "", nil, nil
	}
	socketPath, err := ipc.TempSocketPath()
	if err != nil {
		return "", nil, err
	}
	server, err := ipc.StartServer(socketPath, cache, nil)
	if err != nil {
		_ = os.Remove(socketPath)
		return "", nil, err
	}
	cleanup := func() {
		_ = server.Close()
		_ = os.Remove(socketPath)
	}
	return socketPath, cleanup, nil
}

func runWithPTY(ctx context.Context, cfg config.Config, command *exec.Cmd, cache *cache.Cache, logger *debug.Logger, interactive bool) error {
	command.Env = os.Environ()
	cleanup := func() {}
	if cache != nil {
		socketPath, closeFn, err := startIPCServer(cfg, cache)
		if err != nil {
			fmt.Fprintln(os.Stderr, "secretty: copy cache unavailable:", err)
		} else if socketPath != "" {
			command.Env = append(command.Env, "SECRETTY_SOCKET="+socketPath)
			if closeFn != nil {
				cleanup = closeFn
			}
		}
	}
	defer cleanup()
	if interactive {
		cfg.Redaction.RollingWindowBytes = 0
	}
	detector := detect.NewEngine(cfg)
	stream := redact.NewStream(os.Stdout, cfg, detector, cache, logger)
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
		return nil
	}
	if cfg.Mode == types.ModeStrict && cfg.Strict.DisableCopyOriginal {
		return nil
	}
	ttl := time.Duration(cfg.Overrides.CopyWithoutRender.TTLSeconds) * time.Second
	if existing == nil {
		return cache.New(64, ttl)
	}
	existing.SetTTL(ttl)
	return existing
}

func runDoctor(state *appState) error {
	shell := os.Getenv("SHELL")
	termName := os.Getenv("TERM")
	inTmux := os.Getenv("TMUX") != ""
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		cols = 0
		rows = 0
	}
	fmt.Printf("shell=%s\n", shell)
	fmt.Printf("term=%s\n", termName)
	fmt.Printf("tmux=%t\n", inTmux)
	fmt.Printf("size=%dx%d\n", cols, rows)
	fmt.Printf("config_path=%s\n", state.cfgPath)
	fmt.Printf("config_found=%t\n", state.cfgFound)
	fmt.Printf("mode=%s\n", state.cfg.Mode)
	fmt.Printf("strict_no_reveal=%t\n", state.cfg.Strict.NoReveal)
	fmt.Printf("strict_disable_copy_original=%t\n", state.cfg.Strict.DisableCopyOriginal)
	fmt.Printf("copy_enabled=%t\n", state.cfg.Overrides.CopyWithoutRender.Enabled)
	fmt.Printf("copy_ttl_seconds=%d\n", state.cfg.Overrides.CopyWithoutRender.TTLSeconds)
	fmt.Printf("copy_require_confirm=%t\n", state.cfg.Overrides.CopyWithoutRender.RequireConfirm)
	fmt.Printf("status_line_enabled=%t\n", state.cfg.Redaction.StatusLine.Enabled)
	fmt.Printf("status_line_rate_limit_ms=%d\n", state.cfg.Redaction.StatusLine.RateLimitMS)
	fmt.Printf("rules_enabled=%s\n", strings.Join(enabledRuleNames(state.cfg), ","))
	fmt.Printf("typed_detectors_enabled=%s\n", strings.Join(enabledDetectorNames(state.cfg), ","))
	cacheScope := "in-process"
	if os.Getenv("SECRETTY_SOCKET") != "" {
		cacheScope = "ipc"
	}
	fmt.Printf("cache_scope=%s\n", cacheScope)
	return nil
}

func enabledRuleNames(cfg config.Config) []string {
	var out []string
	for _, rule := range cfg.Rules {
		if rule.Enabled && config.RulesetEnabled(rule.Ruleset, cfg.Rulesets) {
			out = append(out, rule.Name)
		}
	}
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
}

func enabledDetectorNames(cfg config.Config) []string {
	var out []string
	for _, det := range cfg.TypedDetectors {
		if det.Enabled && config.RulesetEnabled(det.Ruleset, cfg.Rulesets) {
			out = append(out, det.Name)
		}
	}
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
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

func envSummary() string {
	shell := os.Getenv("SHELL")
	termName := os.Getenv("TERM")
	inTmux := os.Getenv("TMUX") != ""
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		cols = 0
		rows = 0
	}
	return fmt.Sprintf("Detected shell=%s TERM=%s tmux=%t size=%dx%d", shell, termName, inTmux, cols, rows)
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
