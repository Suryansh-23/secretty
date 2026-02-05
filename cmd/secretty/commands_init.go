package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/detect"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
	"github.com/suryansh-23/secretty/internal/ui"
)

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
			shellBanner := cfg.UI.ShellBanner
			allowlistEnabled := cfg.Allowlist.Enabled
			selectedAllowlist := defaultAllowlistSelections(cfg)
			allowlistCustom := ""
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
					huh.NewConfirm().Title("Show banner when starting a protected shell?").Value(&shellBanner),
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
				huh.NewGroup(
					huh.NewConfirm().Title("Skip redaction for selected commands?").Value(&allowlistEnabled),
				),
				huh.NewGroup(
					huh.NewMultiSelect[string]().Title("Allowlist commands").Value(&selectedAllowlist).Options(
						allowlistOptions()...,
					),
				).WithHideFunc(func() bool { return !allowlistEnabled }),
				huh.NewGroup(
					huh.NewInput().Title("Custom allowlist entries (comma-separated)").Value(&allowlistCustom),
				).WithHideFunc(func() bool { return !allowlistEnabled }),
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
			cfg.UI.ShellBanner = shellBanner
			if copyEnabled {
				parsedTTL, err := strconv.Atoi(strings.TrimSpace(ttlStr))
				if err != nil {
					return fmt.Errorf("parse copy ttl: %w", err)
				}
				cfg.Overrides.CopyWithoutRender.TTLSeconds = parsedTTL
			}
			cfg.Allowlist.Enabled = allowlistEnabled
			cfg.Allowlist.Commands = buildAllowlistCommands(selectedAllowlist, allowlistCustom)
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

func allowlistOptions() []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(allowlistSuggestions()))
	for _, name := range allowlistSuggestions() {
		options = append(options, huh.NewOption(name, name))
	}
	return options
}

func allowlistSuggestions() []string {
	return []string{
		"ssh",
		"vim",
		"less",
		"more",
		"man",
		"top",
		"htop",
		"tail",
		"cat",
		"grep",
		"kubectl*",
		"terraform",
		"aws",
	}
}

func defaultAllowlistSelections(cfg config.Config) []string {
	if len(cfg.Allowlist.Commands) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(cfg.Allowlist.Commands))
	for _, entry := range cfg.Allowlist.Commands {
		set[strings.TrimSpace(entry)] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for _, suggestion := range allowlistSuggestions() {
		if _, ok := set[suggestion]; ok {
			out = append(out, suggestion)
		}
	}
	return out
}

func buildAllowlistCommands(selected []string, custom string) []string {
	seen := make(map[string]struct{}, len(selected))
	out := make([]string, 0, len(selected))
	for _, entry := range selected {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	for _, entry := range strings.Split(custom, ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
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
