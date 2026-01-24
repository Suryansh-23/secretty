package ui

import "github.com/charmbracelet/lipgloss"

const (
	colorCyan   = "#22D3EE"
	colorSky    = "#38BDF8"
	colorBlue   = "#60A5FA"
	colorViolet = "#A78BFA"
	colorPink   = "#F472B6"
	colorRose   = "#FB7185"
	colorMuted  = "#94A3B8"
)

var (
	Primary   = lipgloss.Color(colorCyan)
	Secondary = lipgloss.Color(colorViolet)
	Accent    = lipgloss.Color(colorPink)
	Muted     = lipgloss.Color(colorMuted)
	Palette   = []lipgloss.Color{Primary, lipgloss.Color(colorSky), lipgloss.Color(colorBlue), Secondary, Accent, lipgloss.Color(colorRose)}
)
