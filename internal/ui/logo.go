package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var logoLines = []string{
	`   _____                   ______________  __`,
	`  / ___/___  _____________/_  __/_  __/\ \/ /`,
	`  \__ \/ _ \/ ___/ ___/ _ \/ /   / /    \  / `,
	` ___/ /  __/ /__/ /  /  __/ /   / /     / /  `,
	`/____/\___/\___/_/   \___/_/   /_/     /_/   `,
	`                                             `,
}

// LogoFrame renders a single animated frame.
func LogoFrame(frame int) string {
	lines := make([]string, len(logoLines))
	for i, line := range logoLines {
		color := Palette[(frame+i)%len(Palette)]
		lines[i] = lipgloss.NewStyle().Foreground(color).Render(line)
	}
	return strings.Join(lines, "\n")
}
