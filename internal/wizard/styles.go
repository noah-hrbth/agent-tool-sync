package wizard

import "github.com/charmbracelet/lipgloss"

// standalone palette mirroring internal/tui's colors — the wizard is a
// separate mini-program and must not import the main TUI
var (
	colorPrimary = lipgloss.AdaptiveColor{Light: "#6665DD", Dark: "#6665DD"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#9E9E9E", Dark: "#616161"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#388E3C", Dark: "#66BB6A"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#EF5350"}

	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleSuccess = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
	styleError   = lipgloss.NewStyle().Foreground(colorDanger)
)
