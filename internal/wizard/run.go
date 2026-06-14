package wizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// Run launches the wizard as an inline Bubble Tea program (no alt screen, so
// the final success or error output persists in the terminal) and returns the
// final model's Outcome. The returned error covers program failures only;
// wizard-level failures surface via Outcome.Err.
func Run(ws string, scope tools.Scope, cfg *config.Config, options []SourceOption) (Outcome, error) {
	p := tea.NewProgram(New(ws, scope, cfg, options))
	final, err := p.Run()
	if err != nil {
		return Outcome{}, err
	}
	return final.(Model).Outcome(), nil
}
