package shellconfig

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"strings"
)

const (
	beginMarker = "# >>> secretty >>>"
	endMarker   = "# <<< secretty <<<"
)

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
