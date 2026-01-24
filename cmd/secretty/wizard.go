package main

import (
	"errors"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/suryansh-23/secretty/internal/ui"
)

type tickMsg struct{}

type wizardModel struct {
	form     *huh.Form
	idx      int
	interval time.Duration
}

func newWizardModel(form *huh.Form) wizardModel {
	return wizardModel{
		form:     form,
		interval: 140 * time.Millisecond,
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
	return ui.LogoFrame(m.idx) + "\n\n" + m.form.View()
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{} })
}

func runAnimatedForm(form *huh.Form) error {
	if os.Getenv("TERM") == "dumb" {
		return form.Run()
	}

	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt

	model := newWizardModel(form)
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
