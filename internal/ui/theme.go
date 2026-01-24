package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Theme returns the SecreTTY TUI theme.
func Theme() *huh.Theme {
	t := huh.ThemeBase()

	t.Form.Base = t.Form.Base.PaddingLeft(1)
	t.Group.Title = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	t.Group.Description = lipgloss.NewStyle().Foreground(Muted)

	t.Focused.Title = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	t.Focused.Description = lipgloss.NewStyle().Foreground(Muted)

	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(Primary).SetString("› ")
	t.Focused.MultiSelectSelector = lipgloss.NewStyle().Foreground(Primary).SetString("› ")
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(Primary).SetString("[•] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(Muted).SetString("[ ] ")

	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("0")).Background(Primary)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(Primary).Background(lipgloss.Color("0"))

	t.Blurred.Title = lipgloss.NewStyle().Foreground(Muted)
	t.Blurred.Description = lipgloss.NewStyle().Foreground(Muted)

	t.Focused.NoteTitle = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
	return t
}
