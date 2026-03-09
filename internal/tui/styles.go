package tui

import "github.com/charmbracelet/lipgloss"

var (
	backgroundColor     = lipgloss.Color("#0d1117")
	titleColor          = lipgloss.Color("#f0c674")
	textColor           = lipgloss.Color("#c9d1d9")
	subtleTextColor     = lipgloss.Color("#8b949e")
	activeBorderColor   = lipgloss.Color("#58a6ff")
	inactiveBorderColor = lipgloss.Color("#30363d")

	titleStyle = lipgloss.NewStyle().
			Foreground(titleColor).
			Background(backgroundColor).
			Bold(true).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(subtleTextColor).
			Background(backgroundColor).
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	helpPanelStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(backgroundColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorderColor).
			Padding(1, 2)
)
