package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/suryansh-23/secretty/internal/shellconfig"
)

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
