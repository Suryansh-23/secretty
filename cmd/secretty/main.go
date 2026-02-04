package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	"github.com/suryansh-23/secretty/internal/shellconfig"
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
			return runWithPTY(cmd.Context(), state.cfg, state.cfgPath, command, state.cache, state.logger, true)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&strictFlag, "strict", false, "enable strict mode (no reveal to screen)")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable sanitized debug logging")
	rootCmd.PersistentFlags().BoolVar(&noInitHints, "no-init-hints", false, "suppress init guidance")

	rootCmd.AddCommand(newShellCmd(state))
	rootCmd.AddCommand(newRunCmd(state))
	rootCmd.AddCommand(newInitCmd(&cfgPath))
	rootCmd.AddCommand(newResetCmd(&cfgPath))
	rootCmd.AddCommand(newCopyCmd(state))
	rootCmd.AddCommand(newStatusCmd(state))
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

func newInitCmd(cfgPath *string) *cobra.Command {
	var useDefaults bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Run the first-time setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(*cfgPath)
			if err != nil {
				return err
			}

			cfg := config.DefaultConfig()
			if useDefaults {
				if exists(path) {
					fmt.Printf("Config exists, overwriting: %s\n", path)
				}
				shellOptions := detectShellOptions()
				selectedShells := defaultShellSelections(shellOptions)
				if err := runSelfTest(cfg); err != nil {
					return err
				}
				fmt.Println("Suggested alias: alias safe=secretty")
				if err := config.Write(path, cfg); err != nil {
					return err
				}
				fmt.Printf("Wrote config to %s\n", path)
				if len(selectedShells) > 0 {
					if err := installShellHooks(selectedShells, shellOptions, path); err != nil {
						return err
					}
				}
				return nil
			}
			mode := string(cfg.Mode)
			maskStyle := string(cfg.Masking.Style)
			selectedRulesets := defaultRulesetSelections(cfg)
			shellOptions := detectShellOptions()
			selectedShells := defaultShellSelections(shellOptions)
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
						huh.NewOption("Strict recording (default)", string(types.ModeStrict)),
						huh.NewOption("Demo", string(types.ModeDemo)),
						huh.NewOption("Warn-only", string(types.ModeWarn)),
					),
				),
				huh.NewGroup(
					huh.NewSelect[string]().Title("Redaction style").Value(&maskStyle).Options(
						huh.NewOption("Classic blocks", string(types.MaskStyleBlock)),
						huh.NewOption("Glow blocks (default)", string(types.MaskStyleGlow)),
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
					huh.NewMultiSelect[string]().Title("Install SecreTTY shell hook in").Value(&selectedShells).Options(
						shellOptionsToOptions(shellOptions)...,
					),
				).WithHideFunc(func() bool { return len(shellOptions) == 0 }),
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

			if len(selectedShells) > 0 {
				if err := installShellHooks(selectedShells, shellOptions, path); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useDefaults, "default", false, "write default config without prompts")
	return cmd
}

func newResetCmd(cfgPath *string) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Remove SecreTTY config and shell integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveConfigPath(*cfgPath)
			if err != nil {
				return err
			}
			if !yes {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return errors.New("reset requires --yes when not running interactively")
				}
				confirm := false
				form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Remove SecreTTY config and shell integration?").Value(&confirm)))
				if err := form.Run(); err != nil {
					return err
				}
				if !confirm {
					return errors.New("reset cancelled")
				}
			}

			removedConfig, err := removeConfigFile(path)
			if err != nil {
				return err
			}
			removedBlocks := 0
			for _, shellPath := range defaultShellConfigPaths() {
				changed, err := shellconfig.RemoveBlock(shellPath)
				if err != nil {
					return err
				}
				if changed {
					removedBlocks++
				}
			}

			if removedConfig {
				fmt.Printf("Removed config: %s\n", path)
			} else {
				fmt.Printf("Config not found: %s\n", path)
			}
			if removedBlocks > 0 {
				fmt.Printf("Removed SecreTTY shell blocks from %d file(s)\n", removedBlocks)
			} else {
				fmt.Println("No SecreTTY shell blocks found.")
			}
			fmt.Println("Note: manual aliases or custom shell changes must be removed manually.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompts")
	return cmd
}

func newCopyCmd(state *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy redacted secrets without rendering",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return errors.New("use `secretty copy last` or `secretty copy pick`")
		},
	}
	cmd.AddCommand(newCopyLastCmd(state))
	cmd.AddCommand(newCopyPickCmd(state))
	return cmd
}

func newCopyLastCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "last",
		Short: "Copy the last redacted secret without rendering",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopyLast(state)
		},
	}
}

func newCopyPickCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "pick",
		Short: "Select a cached secret to copy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureCopyAllowed(state); err != nil {
				return err
			}
			entries, err := listCachedSecrets(state)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return errors.New("no secrets cached")
			}
			var selectedID int
			options := make([]huh.Option[int], 0, len(entries))
			for _, entry := range entries {
				label := labelForCopy(entry.Label, entry.RuleName, entry.Type)
				meta := label
				if entry.Type != "" {
					meta = fmt.Sprintf("%s (%s)", label, entry.Type)
				}
				if !entry.CreatedAt.IsZero() {
					age := time.Since(entry.CreatedAt).Truncate(time.Second)
					meta = fmt.Sprintf("%s · %s ago", meta, age)
				}
				options = append(options, huh.NewOption(meta, entry.ID))
			}
			form := huh.NewForm(huh.NewGroup(
				huh.NewSelect[int]().Title("Select secret to copy").Options(options...).Value(&selectedID),
			))
			if err := form.Run(); err != nil {
				return err
			}
			if selectedID == 0 {
				return errors.New("no secret selected")
			}
			if state.cfg.Overrides.CopyWithoutRender.RequireConfirm {
				label := labelForCopyLabel(entries, selectedID)
				confirm := false
				form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(fmt.Sprintf("Copy %s to clipboard?", label)).Value(&confirm)))
				if err := form.Run(); err != nil {
					return err
				}
				if !confirm {
					return errors.New("copy cancelled")
				}
			}
			resp, err := copyByID(state, selectedID)
			if err != nil {
				return err
			}
			printCopyResult(state, resp)
			return nil
		},
	}
}

type copyEntry struct {
	ID        int
	Label     string
	RuleName  string
	Type      types.SecretType
	CreatedAt time.Time
}

type copyResult struct {
	ID       int
	Label    string
	RuleName string
	Type     types.SecretType
}

func ensureCopyAllowed(state *appState) error {
	if !state.cfg.Overrides.CopyWithoutRender.Enabled {
		return errors.New("copy-without-render is disabled")
	}
	if state.cfg.Mode == types.ModeStrict && state.cfg.Strict.DisableCopyOriginal {
		return errors.New("copy original is disabled in strict mode")
	}
	return nil
}

func runCopyLast(state *appState) error {
	if err := ensureCopyAllowed(state); err != nil {
		return err
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
	resp, err := copyLast(state)
	if err != nil {
		return err
	}
	printCopyResult(state, resp)
	return nil
}

func copyLast(state *appState) (copyResult, error) {
	if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
		resp, err := ipc.CopyLast(socketPath)
		if err != nil {
			return copyResult{}, err
		}
		return copyResult{ID: resp.ID, Label: resp.Label, RuleName: resp.RuleName, Type: types.SecretType(resp.Type)}, nil
	}
	if state.cache == nil {
		return copyResult{}, errors.New("no secret cache available")
	}
	record, ok := state.cache.GetLast()
	if !ok {
		return copyResult{}, errors.New("no secrets cached")
	}
	if err := clipboard.CopyBytes(state.cfg.Overrides.CopyWithoutRender.Backend, record.Original); err != nil {
		return copyResult{}, err
	}
	return copyResult{ID: record.ID, Label: record.Label, RuleName: record.RuleName, Type: record.Type}, nil
}

func copyByID(state *appState, id int) (copyResult, error) {
	if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
		resp, err := ipc.CopyByID(socketPath, id)
		if err != nil {
			return copyResult{}, err
		}
		return copyResult{ID: resp.ID, Label: resp.Label, RuleName: resp.RuleName, Type: types.SecretType(resp.Type)}, nil
	}
	if state.cache == nil {
		return copyResult{}, errors.New("no secret cache available")
	}
	record, ok := state.cache.Get(id)
	if !ok {
		return copyResult{}, errors.New("secret not found")
	}
	if err := clipboard.CopyBytes(state.cfg.Overrides.CopyWithoutRender.Backend, record.Original); err != nil {
		return copyResult{}, err
	}
	return copyResult{ID: record.ID, Label: record.Label, RuleName: record.RuleName, Type: record.Type}, nil
}

func listCachedSecrets(state *appState) ([]copyEntry, error) {
	if socketPath := os.Getenv("SECRETTY_SOCKET"); socketPath != "" {
		records, err := ipc.ListSecrets(socketPath)
		if err != nil {
			if errors.Is(err, ipc.ErrUnsupportedOperation) {
				return nil, errors.New("copy pick requires a refreshed SecreTTY wrapper; restart your shell or run `secretty shell` again")
			}
			return nil, err
		}
		out := make([]copyEntry, 0, len(records))
		for _, rec := range records {
			out = append(out, copyEntry{
				ID:        rec.ID,
				Label:     rec.Label,
				RuleName:  rec.RuleName,
				Type:      types.SecretType(rec.Type),
				CreatedAt: rec.CreatedAt,
			})
		}
		return out, nil
	}
	if state.cache == nil {
		return nil, errors.New("no secret cache available")
	}
	records := state.cache.List()
	out := make([]copyEntry, 0, len(records))
	for _, rec := range records {
		out = append(out, copyEntry{
			ID:        rec.ID,
			Label:     rec.Label,
			RuleName:  rec.RuleName,
			Type:      rec.Type,
			CreatedAt: rec.CreatedAt,
		})
	}
	return out, nil
}

func labelForCopy(label, rule string, secretType types.SecretType) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	rule = strings.TrimSpace(rule)
	if rule != "" {
		return rule
	}
	if secretType != "" {
		return string(secretType)
	}
	return "secret"
}

func labelForCopyLabel(entries []copyEntry, id int) string {
	for _, entry := range entries {
		if entry.ID == id {
			return labelForCopy(entry.Label, entry.RuleName, entry.Type)
		}
	}
	return "secret"
}

func printCopyResult(state *appState, resp copyResult) {
	label := labelForCopy(resp.Label, resp.RuleName, resp.Type)
	if state.cfg.Redaction.IncludeEventID && resp.ID > 0 {
		fmt.Printf("Copied %s (%d) to clipboard\n", label, resp.ID)
		return
	}
	fmt.Printf("Copied %s to clipboard\n", label)
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

func newStatusCmd(state *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print SecreTTY wrapper status",
		RunE: func(cmd *cobra.Command, args []string) error {
			wrapped := os.Getenv("SECRETTY_WRAPPED") != ""
			socket := os.Getenv("SECRETTY_SOCKET") != ""
			fmt.Printf("wrapped=%t\n", wrapped)
			fmt.Printf("ipc_socket=%t\n", socket)
			if envCfg := strings.TrimSpace(os.Getenv("SECRETTY_CONFIG")); envCfg != "" {
				fmt.Printf("config=%s\n", envCfg)
			} else {
				fmt.Printf("config=%s\n", state.cfgPath)
			}
			return nil
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

type shellOption struct {
	Name string
	Kind string
	Path string
}

func detectShellOptions() []shellOption {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := defaultShellCandidates(home)
	current := filepath.Base(os.Getenv("SHELL"))
	etcShells := readEtcShells()
	var out []shellOption
	for _, candidate := range candidates {
		if candidate.Kind == current || exists(candidate.Path) || etcShells[candidate.Kind] || hasInPath(candidate.Kind) {
			out = append(out, candidate)
		}
	}
	if len(out) == 0 {
		return candidates
	}
	return out
}

func defaultShellCandidates(home string) []shellOption {
	fishPath := filepath.Join(home, ".config", "fish", "conf.d", "secretty.fish")
	if runtime.GOOS == "linux" {
		return []shellOption{
			{Name: "bash", Kind: "bash", Path: filepath.Join(home, ".bashrc")},
			{Name: "zsh", Kind: "zsh", Path: filepath.Join(home, ".zshrc")},
			{Name: "fish", Kind: "fish", Path: fishPath},
		}
	}
	return []shellOption{
		{Name: "zsh", Kind: "zsh", Path: filepath.Join(home, ".zshenv")},
		{Name: "bash", Kind: "bash", Path: filepath.Join(home, ".bash_profile")},
		{Name: "fish", Kind: "fish", Path: fishPath},
	}
}

func defaultShellSelections(options []shellOption) []string {
	current := filepath.Base(os.Getenv("SHELL"))
	var out []string
	for _, opt := range options {
		if opt.Kind == current {
			out = append(out, opt.Kind)
		}
	}
	return out
}

func shellOptionsToOptions(options []shellOption) []huh.Option[string] {
	out := make([]huh.Option[string], 0, len(options))
	for _, opt := range options {
		label := fmt.Sprintf("%s (%s)", opt.Name, opt.Path)
		out = append(out, huh.NewOption(label, opt.Kind))
	}
	return out
}

func installShellHooks(selected []string, options []shellOption, configPath string) error {
	lookup := make(map[string]shellOption, len(options))
	for _, opt := range options {
		lookup[opt.Kind] = opt
	}
	binPath := resolveSecrettyBinary()
	for _, kind := range selected {
		opt, ok := lookup[kind]
		if !ok {
			continue
		}
		changed, err := shellconfig.InstallBlock(opt.Path, opt.Kind, configPath, binPath)
		if err != nil {
			return err
		}
		if changed {
			fmt.Printf("Installed shell hook in %s\n", opt.Path)
		}
	}
	return nil
}

func resolveSecrettyBinary() string {
	exe, _ := os.Executable()
	exe = filepath.Clean(exe)
	if isExecutableFile(exe) && isStableExecutablePath(exe) {
		return exe
	}
	if resolved, err := exec.LookPath("secretty"); err == nil {
		resolved = filepath.Clean(resolved)
		if isExecutableFile(resolved) && isStableExecutablePath(resolved) {
			return resolved
		}
	}
	if isExecutableFile(exe) {
		return exe
	}
	if resolved, err := exec.LookPath("secretty"); err == nil && isExecutableFile(resolved) {
		return filepath.Clean(resolved)
	}
	return ""
}

func isStableExecutablePath(path string) bool {
	if path == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	tempDir := filepath.Clean(os.TempDir())
	if tempDir != "." && tempDir != "/" && strings.HasPrefix(cleaned, tempDir+string(os.PathSeparator)) {
		return false
	}
	if strings.HasPrefix(cleaned, "/var/folders/") || strings.HasPrefix(cleaned, "/private/var/folders/") {
		return false
	}
	if strings.HasPrefix(cleaned, "/tmp/") {
		return false
	}
	return true
}

func isExecutableFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

func defaultShellConfigPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".zshenv"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".zprofile"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".config", "fish", "config.fish"),
		filepath.Join(home, ".config", "fish", "conf.d", "secretty.fish"),
	}
}

func readEtcShells() map[string]bool {
	data, err := os.ReadFile("/etc/shells")
	if err != nil {
		return map[string]bool{}
	}
	out := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[filepath.Base(line)] = true
	}
	return out
}

func hasInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func removeConfigFile(path string) (bool, error) {
	if !exists(path) {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	dir := filepath.Dir(path)
	if isDirEmpty(dir) {
		_ = os.Remove(dir)
	}
	return true, nil
}

func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
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
	server, err := ipc.StartServer(socketPath, cache, func(payload []byte) error {
		return clipboard.CopyBytes(cfg.Overrides.CopyWithoutRender.Backend, payload)
	})
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

func runWithPTY(ctx context.Context, cfg config.Config, cfgPath string, command *exec.Cmd, cache *cache.Cache, logger *debug.Logger, interactive bool) error {
	command.Env = os.Environ()
	if os.Getenv("SECRETTY_HOOK_DEBUG") != "" {
		stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
		stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))
		fmt.Fprintf(os.Stderr, "secretty wrapper: interactive=%t stdin_tty=%t stdout_tty=%t cfg=%s cmd=%s\n", interactive, stdinTTY, stdoutTTY, cfgPath, strings.Join(command.Args, " "))
	}
	if os.Getenv("SECRETTY_WRAPPED") == "" {
		command.Env = append(command.Env, "SECRETTY_WRAPPED=1")
	}
	if cfgPath != "" && os.Getenv("SECRETTY_CONFIG") == "" {
		command.Env = append(command.Env, "SECRETTY_CONFIG="+cfgPath)
	}
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
	exitCode, err := ptywrap.RunCommand(ctx, command, ptywrap.Options{
		RawMode: true,
		Output:  stream,
		Logger:  logger,
	})
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
	if env := strings.TrimSpace(os.Getenv("SECRETTY_CONFIG")); env != "" {
		return env, nil
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
