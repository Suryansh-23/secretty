package shellconfig

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "# >>> secretty >>>"
	endMarker   = "# <<< secretty <<<"
)

// InstallBlock removes any existing block and appends a new one.
func InstallBlock(path, shellKind, configPath, binPath string) (bool, error) {
	block, err := blockForShell(shellKind, configPath, binPath)
	if err != nil {
		return false, err
	}
	_, err = RemoveBlock(path)
	if err != nil {
		return false, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	content, perm, err := readFileWithPerm(path)
	if err != nil {
		return false, err
	}
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		content = append(content, '\n')
	}
	content = append(content, []byte(strings.Join(block, "\n")+"\n")...)
	if err := os.WriteFile(path, content, perm); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveBlock removes the SecreTTY marker block from a shell config file.
func RemoveBlock(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	changed := false
	inBlock := false
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, beginMarker) {
			inBlock = true
			changed = true
			continue
		}
		if strings.Contains(line, endMarker) {
			if inBlock {
				inBlock = false
				changed = true
				continue
			}
		}
		if inBlock {
			changed = true
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	output := strings.Join(lines, "\n")
	if bytes.HasSuffix(data, []byte("\n")) {
		output += "\n"
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(output), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

func blockForShell(kind, configPath, binPath string) ([]string, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, errors.New("config path required")
	}
	binPath = strings.TrimSpace(binPath)
	switch kind {
	case "zsh":
		return []string{
			beginMarker,
			"if [[ -o interactive ]] && [[ -z \"$SECRETTY_WRAPPED\" ]]; then",
			"  if [[ -r /dev/tty ]]; then",
			"    secretty_bin=\"\"",
			fmt.Sprintf("    if [[ -n \"%s\" && -x \"%s\" ]]; then", binPath, binPath),
			fmt.Sprintf("      secretty_bin=\"%s\"", binPath),
			"    elif command -v secretty >/dev/null 2>&1; then",
			"      secretty_bin=\"$(command -v secretty)\"",
			"    fi",
			"    if [[ -n \"$SECRETTY_HOOK_DEBUG\" ]]; then",
			"      echo \"secretty hook: shell=zsh interactive=$- wrapped=$SECRETTY_WRAPPED tty_ok=1 bin=$secretty_bin\" >&2",
			"    fi",
			"    if [[ -n \"$secretty_bin\" ]]; then",
			fmt.Sprintf("      export SECRETTY_CONFIG=\"%s\"", configPath),
			"      exec \"$secretty_bin\" </dev/tty >/dev/tty 2>/dev/tty",
			"    fi",
			"  elif [[ -n \"$SECRETTY_HOOK_DEBUG\" ]]; then",
			"    echo \"secretty hook: shell=zsh interactive=$- wrapped=$SECRETTY_WRAPPED tty_ok=0\" >&2",
			"  fi",
			"fi",
			endMarker,
		}, nil
	case "bash", "sh":
		return []string{
			beginMarker,
			"case $- in",
			"  *i*)",
			"    if [ -z \"$SECRETTY_WRAPPED\" ]; then",
			"      if [ -r /dev/tty ]; then",
			"        secretty_bin=\"\"",
			fmt.Sprintf("        if [ -n \"%s\" ] && [ -x \"%s\" ]; then", binPath, binPath),
			fmt.Sprintf("          secretty_bin=\"%s\"", binPath),
			"        elif command -v secretty >/dev/null 2>&1; then",
			"          secretty_bin=\"$(command -v secretty)\"",
			"        fi",
			"        if [ -n \"$SECRETTY_HOOK_DEBUG\" ]; then",
			"          echo \"secretty hook: shell=bash interactive=$- wrapped=$SECRETTY_WRAPPED tty_ok=1 bin=$secretty_bin\" >&2",
			"        fi",
			"        if [ -n \"$secretty_bin\" ]; then",
			fmt.Sprintf("          export SECRETTY_CONFIG=\"%s\"", configPath),
			"          exec \"$secretty_bin\" </dev/tty >/dev/tty 2>/dev/tty",
			"        fi",
			"      elif [ -n \"$SECRETTY_HOOK_DEBUG\" ]; then",
			"        echo \"secretty hook: shell=bash interactive=$- wrapped=$SECRETTY_WRAPPED tty_ok=0\" >&2",
			"      fi",
			"    fi",
			"    ;;",
			"esac",
			endMarker,
		}, nil
	case "fish":
		return []string{
			beginMarker,
			"if status --is-interactive; and not set -q SECRETTY_WRAPPED",
			"  if test -r /dev/tty",
			"    set -l secretty_bin \"\"",
			fmt.Sprintf("    if test -n \"%s\" -a -x \"%s\"", binPath, binPath),
			fmt.Sprintf("      set secretty_bin \"%s\"", binPath),
			"    else if type -q secretty",
			"      set secretty_bin (command -v secretty)",
			"    end",
			"    if set -q SECRETTY_HOOK_DEBUG",
			"      echo \"secretty hook: shell=fish wrapped=$SECRETTY_WRAPPED tty_ok=1 bin=$secretty_bin\" >&2",
			"    end",
			"    if test -n \"$secretty_bin\"",
			fmt.Sprintf("      set -gx SECRETTY_CONFIG \"%s\"", configPath),
			"      exec $secretty_bin </dev/tty >/dev/tty 2>/dev/tty",
			"    end",
			"  else if set -q SECRETTY_HOOK_DEBUG",
			"    echo \"secretty hook: shell=fish wrapped=$SECRETTY_WRAPPED tty_ok=0\" >&2",
			"  end",
			"end",
			endMarker,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported shell: %s", kind)
	}
}

func readFileWithPerm(path string) ([]byte, os.FileMode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0o644, nil
		}
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	return data, info.Mode().Perm(), nil
}
