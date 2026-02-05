package main

import (
	"errors"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/suryansh-23/secretty/internal/ui"
)

type tickMsg struct{}

type wizardModel struct {
	form     *huh.Form
	idx      int
	interval time.Duration
	palette  []lipgloss.Color
	badge    ui.Badge
}

func newWizardModel(form *huh.Form, palette []lipgloss.Color, badge ui.Badge) wizardModel {
	return wizardModel{
		form:     form,
		interval: 140 * time.Millisecond,
		palette:  palette,
		badge:    badge,
	}
}

func (m wizardModel) Init() tea.Cmd {
	return tea.Batch(m.form.Init(), tick(m.interval))
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		m.idx = (m.idx + 1) % len(ui.Palette)
		return m, tick(m.interval)
	}

	model, cmd := m.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m wizardModel) View() string {
	formView := compactFooterSpacing(m.form.View())
	return ui.LogoFrame(m.idx, m.palette, m.badge) + "\n\n" + formView
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{} })
}

func compactFooterSpacing(view string) string {
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		return view
	}
	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	if last < 0 {
		return view
	}
	footerStart := last
	for footerStart-1 >= 0 && strings.TrimSpace(lines[footerStart-1]) != "" {
		footerStart--
	}
	contentEnd := footerStart - 1
	for contentEnd >= 0 && strings.TrimSpace(lines[contentEnd]) == "" {
		contentEnd--
	}
	if contentEnd < 0 {
		return strings.Join(lines[footerStart:last+1], "\n")
	}
	compact := append([]string{}, lines[:contentEnd+1]...)
	compact = append(compact, "")
	compact = append(compact, lines[footerStart:last+1]...)
	return strings.Join(compact, "\n")
}

func runAnimatedForm(form *huh.Form) error {
	if os.Getenv("TERM") == "dumb" {
		return form.Run()
	}

	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt

	model := newWizardModel(form, ui.ShuffledPalette(), currentBadge())
	p := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithInput(os.Stdin), tea.WithReportFocus())
	m, err := p.Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return huh.ErrUserAborted
		}
		return err
	}
	switch wm := m.(type) {
	case wizardModel:
		if wm.form.State == huh.StateAborted {
			return huh.ErrUserAborted
		}
	case *wizardModel:
		if wm.form.State == huh.StateAborted {
			return huh.ErrUserAborted
		}
	}
	return nil
}
