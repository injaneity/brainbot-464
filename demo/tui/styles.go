package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
const (
	colorPrimary   = "#7D56F4"
	colorSuccess   = "#04B575"
	colorError     = "#FF0000"
	colorInfo      = "#626262"
	colorHighlight = "#FAFAFA"
	colorBorder    = "#874BFD"
)

// Styles for the TUI application
var (
	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorPrimary)).
		MarginTop(1).
		MarginBottom(1)

	StatusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorSuccess))

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorError))

	InfoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorInfo))

	BoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 2)

	HighlightStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorHighlight)).
		Background(lipgloss.Color(colorPrimary)).
		Padding(0, 1)
)
