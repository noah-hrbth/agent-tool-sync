// Package wizard implements the first-run init wizard: a standalone Bubble
// Tea mini-program (separate from the main TUI) that initializes an
// uninitialized scope either from scratch or by importing from a detected
// tool's existing configuration.
package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// step identifies the wizard's current screen.
type step int

const (
	stepMethod step = iota
	stepTool
	stepRunning
	stepDone
	stepError
)

// methodChoices are the stepMethod rows; import is listed first because the
// wizard is only launched when at least one import option exists.
var methodChoices = [...]string{"Import from a detected tool", "Start fresh"}

const (
	methodImport = 0
	methodFresh  = 1
)

// Outcome reports what the wizard did once its program exits.
type Outcome struct {
	Imported bool
	ToolName string // source tool when Imported
	Summary  syncer.ImportSummary
	Aborted  bool
	Err      error
}

// Model is the wizard's Bubble Tea model. Construct it with New and read the
// result with Outcome after the program finishes.
type Model struct {
	ws      string
	scope   tools.Scope
	cfg     *config.Config
	options []SourceOption

	step      step
	methodIdx int
	toolIdx   int
	spin      spinner.Model
	importing string // source tool name while stepRunning imports

	outcome Outcome
}

// New returns a wizard model for the given scope base dir, scope, config to
// persist on scaffold, and import options (typically from BuildOptions).
// Callers must pass at least one option — the cmd layer never launches the
// wizard with zero options; it scaffolds directly instead.
func New(ws string, scope tools.Scope, cfg *config.Config, options []SourceOption) Model {
	return Model{
		ws:      ws,
		scope:   scope,
		cfg:     cfg,
		options: options,
		spin:    spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

// Outcome returns what the wizard did; meaningful once the program has quit.
func (m Model) Outcome() Outcome { return m.outcome }

// Init implements tea.Model; the wizard starts idle at the method step.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case freshDoneMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		m.step = stepDone
		return m, tea.Quit
	case importDoneMsg:
		if msg.err != nil {
			return m.fail(msg.err)
		}
		m.step = stepDone
		m.outcome.Imported = true
		m.outcome.ToolName = m.importing
		m.outcome.Summary = msg.summary
		return m, tea.Quit
	case spinner.TickMsg:
		if m.step != stepRunning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleKey routes a keypress to the current step's handler.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.step {
	case stepMethod:
		return m.handleMethodKey(msg)
	case stepTool:
		return m.handleToolKey(msg)
	case stepError:
		return m, tea.Quit // any key exits
	default:
		// keys ignored while running: the scaffold/import cmd cannot be
		// cancelled, so quitting here would falsely report an abort
		return m, nil
	}
}

// handleMethodKey handles navigation and selection at the method step.
func (m Model) handleMethodKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "ctrl+c":
		m.outcome.Aborted = true
		return m, tea.Quit
	case "up", "k":
		if m.methodIdx > 0 {
			m.methodIdx--
		}
	case "down", "j":
		if m.methodIdx < len(methodChoices)-1 {
			m.methodIdx++
		}
	case "enter":
		return m.selectMethod()
	}
	return m, nil
}

// selectMethod transitions out of the method step: import → tool step,
// fresh → scaffold immediately.
func (m Model) selectMethod() (tea.Model, tea.Cmd) {
	if m.methodIdx == methodImport {
		m.step = stepTool
		return m, nil
	}
	m.step = stepRunning
	return m, tea.Batch(m.spin.Tick, scaffoldFreshCmd(m.ws, m.scope, m.cfg))
}

// handleToolKey handles navigation and selection at the tool step.
func (m Model) handleToolKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.step = stepMethod
	case "q", "ctrl+c":
		m.outcome.Aborted = true
		return m, tea.Quit
	case "up", "k":
		if m.toolIdx > 0 {
			m.toolIdx--
		}
	case "down", "j":
		if m.toolIdx < len(m.options)-1 {
			m.toolIdx++
		}
	case "enter":
		return m.selectTool()
	}
	return m, nil
}

// selectTool starts the scaffold-then-import flow for the highlighted option.
func (m Model) selectTool() (tea.Model, tea.Cmd) {
	opt := m.options[m.toolIdx]
	m.step = stepRunning
	m.importing = opt.Tool.Meta.Name
	return m, tea.Batch(m.spin.Tick, importFromToolCmd(m.ws, m.scope, m.cfg, opt.Tool))
}

// fail moves the wizard to the error step and records err on the outcome.
func (m Model) fail(err error) (tea.Model, tea.Cmd) {
	m.step = stepError
	m.outcome.Err = err
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.step {
	case stepMethod:
		return m.viewMethod()
	case stepTool:
		return m.viewTool()
	case stepRunning:
		return m.viewRunning()
	case stepDone:
		return m.viewDone()
	case stepError:
		return m.viewError()
	}
	return ""
}

// viewMethod renders the method-selection step.
func (m Model) viewMethod() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("Initialize agentsync (%s scope)", m.scope)) + "\n\n")
	for i, choice := range methodChoices {
		b.WriteString(cursorRow(choice, i == m.methodIdx) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("↑/↓ move · enter select · esc quit") + "\n")
	return b.String()
}

// viewTool renders the import-source selection step.
func (m Model) viewTool() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Import from which tool?") + "\n\n")
	for i, opt := range m.options {
		b.WriteString(cursorRow(optionLabel(opt), i == m.toolIdx) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("↑/↓ move · enter select · esc back") + "\n")
	return b.String()
}

// viewRunning renders the in-progress step.
func (m Model) viewRunning() string {
	if m.importing == "" {
		return m.spin.View() + "Initializing .agentsync/…\n"
	}
	return m.spin.View() + "Importing from " + m.importing + "…\n"
}

// viewDone renders the success step.
func (m Model) viewDone() string {
	success := "✓ Initialized .agentsync/"
	if m.outcome.Imported {
		success += " from " + m.outcome.ToolName
	}
	lines := []string{styleSuccess.Render(success)}
	if m.outcome.Imported {
		lines = append(lines, syncer.FormatImportSummary(m.outcome.Summary))
	}
	lines = append(lines, fmt.Sprintf("Run '%s' to start syncing.", m.scope.Command()))
	return strings.Join(lines, "\n") + "\n"
}

// viewError renders the failure step.
func (m Model) viewError() string {
	return styleError.Render(fmt.Sprintf("✗ %v", m.outcome.Err)) + "\n" +
		styleMuted.Render("press any key to exit") + "\n"
}

// cursorRow renders one selectable row with a cursor marker when selected.
func cursorRow(label string, selected bool) string {
	if selected {
		return styleCursor.Render("> " + label)
	}
	return "  " + label
}

// optionLabel renders a source option's display name, suffixing the
// recommended marker.
func optionLabel(o SourceOption) string {
	if o.Recommended {
		return o.Tool.Meta.Name + " (recommended)"
	}
	return o.Tool.Meta.Name
}
