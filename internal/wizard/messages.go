package wizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// freshDoneMsg reports the result of scaffolding a fresh canonical source.
type freshDoneMsg struct{ err error }

// importDoneMsg reports the result of scaffolding plus importing from a
// detected source tool.
type importDoneMsg struct {
	summary syncer.ImportSummary
	err     error
}

// scaffoldFreshCmd returns a command that scaffolds .agentsync/ under ws and
// reports completion as a freshDoneMsg.
func scaffoldFreshCmd(ws string, scope tools.Scope, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		return freshDoneMsg{err: syncer.Scaffold(ws, scope, cfg)}
	}
}

// importFromToolCmd returns a command that scaffolds .agentsync/ under ws and
// then imports t's existing config, reporting an importDoneMsg. Scaffold runs
// first so the import overwrites the starter AGENTS.md only when the source
// actually has root memory.
func importFromToolCmd(ws string, scope tools.Scope, cfg *config.Config, t tools.Tool) tea.Cmd {
	return func() tea.Msg {
		if err := syncer.Scaffold(ws, scope, cfg); err != nil {
			return importDoneMsg{err: err}
		}
		summary, err := syncer.ImportFromTool(ws, t, scope)
		return importDoneMsg{summary: summary, err: err}
	}
}
