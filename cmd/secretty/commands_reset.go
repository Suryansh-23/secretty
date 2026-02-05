package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/suryansh-23/secretty/internal/shellconfig"
)

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
