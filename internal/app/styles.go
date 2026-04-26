package app

import "github.com/charmbracelet/lipgloss"

// Styles contains semantic rendering tokens for the workbench UI.
type Styles struct {
	Title           lipgloss.Style
	Key             lipgloss.Style
	KeyDescription  lipgloss.Style
	Divider         lipgloss.Style
	FrameBorder     lipgloss.Style
	PaneBorder      lipgloss.Style
	PaneTitle       lipgloss.Style
	PaneTitleActive lipgloss.Style
	Selected        lipgloss.Style
	SelectedMuted   lipgloss.Style
	Muted           lipgloss.Style
	Success         lipgloss.Style
	Warning         lipgloss.Style
	Danger          lipgloss.Style
}

// DefaultStyles returns the built-in semantic styles.
func DefaultStyles() Styles {
	return Styles{
		FrameBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7FF")),
	}
}
