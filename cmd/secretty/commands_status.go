package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/config"
)

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

func runDoctor(state *appState) error {
	info := readEnvInfo()
	fmt.Printf("shell=%s\n", info.shell)
	fmt.Printf("term=%s\n", info.term)
	fmt.Printf("tmux=%t\n", info.tmux)
	fmt.Printf("size=%dx%d\n", info.cols, info.rows)
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
