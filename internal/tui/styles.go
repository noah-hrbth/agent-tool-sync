package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.AdaptiveColor{Light: "#6665DD", Dark: "#6665DD"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#9E9E9E", Dark: "#616161"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#388E3C", Dark: "#66BB6A"}
	colorWarn    = lipgloss.AdaptiveColor{Light: "#F57C00", Dark: "#FFA726"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#EF5350"}

	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorMuted)

	stylePanelBorderInset = stylePanelBorder.
				Padding(0, 1).
				MarginLeft(1)

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
			Padding(0, 1).
			MarginLeft(2)

	styleFooterKey   = lipgloss.NewStyle().Bold(true)
	styleFooterLabel = lipgloss.NewStyle().Foreground(colorMuted)

	styleIconSynced      = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	styleIconDivergent   = lipgloss.NewStyle().Foreground(colorWarn).Render("▲")
	styleIconMissing     = lipgloss.NewStyle().Foreground(colorDanger).Render("○")
	styleIconNew         = lipgloss.NewStyle().Foreground(colorMuted).Render("+")
	styleIconPlaceholder = lipgloss.NewStyle().Foreground(colorMuted).Render("·")
	stylePlaceholderRow  = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	styleInputError      = lipgloss.NewStyle().Foreground(colorDanger)
	styleInputLabel      = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	styleBadgeOk   = lipgloss.NewStyle().Foreground(colorSuccess).Render("✓")
	styleBadgeFail = lipgloss.NewStyle().Foreground(colorDanger).Render("✗")
	styleBadgeWarn = lipgloss.NewStyle().Foreground(colorWarn).Render("⚠")

	styleModalBorder = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2)

	styleFileGroupHeader = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)

	styleSyncToolHeader = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleSyncConcept    = lipgloss.NewStyle().Foreground(colorMuted)
	styleSyncSummary    = lipgloss.NewStyle().Bold(true).Foreground(colorSuccess)
)
