package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

type envInfo struct {
	shell string
	term  string
	tmux  bool
	cols  int
	rows  int
}

func readEnvInfo() envInfo {
	info := envInfo{
		shell: os.Getenv("SHELL"),
		term:  os.Getenv("TERM"),
		tmux:  os.Getenv("TMUX") != "",
	}
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		cols = 0
		rows = 0
	}
	info.cols = cols
	info.rows = rows
	return info
}

func envSummary() string {
	info := readEnvInfo()
	return fmt.Sprintf("Detected shell=%s TERM=%s tmux=%t size=%dx%d", info.shell, info.term, info.tmux, info.cols, info.rows)
}
