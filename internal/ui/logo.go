package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var logoLines = []string{
	`   _____                   ______________  __`,
	`  / ___/___  _____________/_  __/_  __/\ \/ /`,
	`  \__ \/ _ \/ ___/ ___/ _ \/ /   / /    \  / `,
	` ___/ /  __/ /__/ /  /  __/ /   / /     / /  `,
	`/____/\___/\___/_/   \___/_/   /_/     /_/   `,
}

// Badge describes the platform + shell label.
type Badge struct {
	Platform string
	Shell    string
}

// LogoFrame renders a single animated frame.
func LogoFrame(frame int, palette []lipgloss.Color, badge Badge) string {
	lines := make([]string, len(logoLines))
	for i, line := range logoLines {
		color := palette[(frame+i)%len(palette)]
		lines[i] = lipgloss.NewStyle().Foreground(color).Render(line)
	}
	return strings.Join(append(lines, badgeLines(frame, palette, badge)...), "\n")
}

// LogoStatic renders a non-animated logo.
func LogoStatic(badge Badge) string {
	return LogoFrame(0, ShuffledPalette(), badge)
}

// ShuffledPalette returns a randomized palette ordering.
func ShuffledPalette() []lipgloss.Color {
	out := make([]lipgloss.Color, len(Palette))
	copy(out, Palette)
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	rnd.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func badgeLines(frame int, palette []lipgloss.Color, badge Badge) []string {
	platform := strings.TrimSpace(badge.Platform)
	shell := strings.TrimSpace(badge.Shell)
	if platform == "" {
		platform = "unknown"
	}
	if shell == "" {
		shell = "shell"
	}
	label := fmt.Sprintf("%s / %s", platform, shell)
	color := palette[(frame+len(logoLines))%len(palette)]
	style := lipgloss.NewStyle().Foreground(color)
	return []string{
		style.Render(logoIndent() + "-- " + label + " --"),
	}
}

// WrapIndicatorLine renders a small indicator for wrapped sessions.
func WrapIndicatorLine(frame int, palette []lipgloss.Color) string {
	color := palette[(frame+len(logoLines)+1)%len(palette)]
	style := lipgloss.NewStyle().Foreground(color)
	return style.Render(logoIndent() + "• wrapped session")
}

func logoIndent() string {
	min := -1
	for _, line := range logoLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " "))
		if min == -1 || leading < min {
			min = leading
		}
	}
	if min <= 0 {
		return ""
	}
	return strings.Repeat(" ", min)
}
