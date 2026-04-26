package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.AdaptiveColor{Light: "#5C6BC0", Dark: "#7986CB"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#9E9E9E", Dark: "#616161"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#388E3C", Dark: "#66BB6A"}
	colorWarn    = lipgloss.AdaptiveColor{Light: "#F57C00", Dark: "#FFA726"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#EF5350"}

	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorMuted)

	stylePanelBorderActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	styleTab = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorMuted)

	styleTabActive = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(colorPrimary).
			Underline(true)

	styleCursorMark = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleSelected   = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	styleFooter = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	styleIconSynced    = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	styleIconDivergent = lipgloss.NewStyle().Foreground(colorWarn).Render("▲")
	styleIconMissing   = lipgloss.NewStyle().Foreground(colorDanger).Render("○")
	styleIconNew       = lipgloss.NewStyle().Foreground(colorMuted).Render("+")

	styleBadgeOk   = lipgloss.NewStyle().Foreground(colorSuccess).Render("✓")
	styleBadgeFail = lipgloss.NewStyle().Foreground(colorDanger).Render("✗")
	styleBadgeWarn = lipgloss.NewStyle().Foreground(colorWarn).Render("⚠")

	styleModalBorder = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorWarn).
				Padding(1, 2)

	styleSyncToolHeader = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleSyncConcept    = lipgloss.NewStyle().Foreground(colorMuted)
	styleSyncSummary    = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
)
