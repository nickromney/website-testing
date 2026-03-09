package tui

import "github.com/charmbracelet/lipgloss"

var (
	backgroundColour     = lipgloss.Color("#0d1117") //nolint:misspell // lipgloss API uses American spelling.
	titleColour          = lipgloss.Color("#f0c674") //nolint:misspell // lipgloss API uses American spelling.
	textColour           = lipgloss.Color("#c9d1d9") //nolint:misspell // lipgloss API uses American spelling.
	subtleTextColour     = lipgloss.Color("#8b949e") //nolint:misspell // lipgloss API uses American spelling.
	activeBorderColour   = lipgloss.Color("#58a6ff") //nolint:misspell // lipgloss API uses American spelling.
	inactiveBorderColour = lipgloss.Color("#30363d") //nolint:misspell // lipgloss API uses American spelling.

	titleStyle = lipgloss.NewStyle().
			Foreground(titleColour).
			Background(backgroundColour).
			Bold(true).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(subtleTextColour).
			Background(backgroundColour).
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Foreground(textColour).
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	helpPanelStyle = lipgloss.NewStyle().
			Foreground(textColour).
			Background(backgroundColour).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorderColour).
			Padding(1, 2)
)
