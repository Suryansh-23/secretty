package main

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/suryansh-23/secretty/internal/ui"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), ui.LogoStatic(currentBadge()))
			fmt.Fprintln(cmd.OutOrStdout())
			ver, rev, built := resolveVersion()
			fmt.Printf("secretty %s\n", ver)
			if rev != "" && rev != "unknown" {
				fmt.Printf("commit %s\n", rev)
			}
			if built != "" && built != "unknown" {
				fmt.Printf("built %s\n", built)
			}
		},
	}
}

func resolveVersion() (string, string, string) {
	ver := strings.TrimSpace(version)
	rev := strings.TrimSpace(commit)
	built := strings.TrimSpace(date)
	if info, ok := debug.ReadBuildInfo(); ok {
		if ver == "" || ver == "dev" {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				ver = info.Main.Version
			}
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if rev == "" || rev == "unknown" {
					rev = setting.Value
				}
			case "vcs.time":
				if built == "" || built == "unknown" {
					built = setting.Value
				}
			}
		}
	}
	if ver == "" {
		ver = "dev"
	}
	return ver, rev, built
}
