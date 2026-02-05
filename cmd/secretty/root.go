package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/debug"
	"github.com/suryansh-23/secretty/internal/types"
)

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
	rootCmd.AddCommand(newVersionCmd())

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
