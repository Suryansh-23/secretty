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
func InstallBlock(path, shellKind, configPath string) (bool, error) {
	block, err := blockForShell(shellKind, configPath)
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

func blockForShell(kind, configPath string) ([]string, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return nil, errors.New("config path required")
	}
	switch kind {
	case "zsh", "bash", "sh":
		return []string{
			beginMarker,
			"if [[ -o interactive ]] && [[ -t 0 ]] && [[ -z \"$SECRETTY_WRAPPED\" ]]; then",
			"  if command -v secretty >/dev/null 2>&1; then",
			fmt.Sprintf("    export SECRETTY_CONFIG=\"%s\"", configPath),
			"    if [[ -n \"$SECRETTY_AUTOEXEC\" ]]; then",
			"      exec secretty",
			"    else",
			"      secretty || echo \"secretty: failed to start; continuing without wrapper\" >&2",
			"    fi",
			"  fi",
			"fi",
			endMarker,
		}, nil
	case "fish":
		return []string{
			beginMarker,
			"if status --is-interactive; and test -t 0; and not set -q SECRETTY_WRAPPED",
			"  if type -q secretty",
			fmt.Sprintf("    set -gx SECRETTY_CONFIG \"%s\"", configPath),
			"    if set -q SECRETTY_AUTOEXEC",
			"      exec secretty",
			"    else",
			"      secretty; or echo \"secretty: failed to start; continuing without wrapper\" >&2",
			"    end",
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
