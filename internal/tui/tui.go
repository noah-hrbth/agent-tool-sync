package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// ---- types ----

type screen int

const (
	screenFiles screen = iota
	screenTools
	screenSync
)

type divChoice int

const (
	choiceNone      divChoice = iota
	choiceAdopt               // use external edit, update canonical
	choiceOverwrite           // discard external edit, write canonical
	choiceDefer               // skip this file this run
)

type fileKind int

const (
	kindRules fileKind = iota
	kindSkill
	kindAgent
	kindCommand
)

type fileItem struct {
	label   string
	kind    fileKind
	skill   *canonical.Skill
	agent   *canonical.Agent
	command *canonical.Command
}

type toolItem struct {
	adapter tools.Adapter
	enabled bool
	install tools.Installation
}

// ---- tea.Msg types ----

type statusResultMsg struct{ results []syncer.FileResult }
type syncDoneMsg struct {
	result *syncer.SyncResult
	err    error
}
type reloadMsg struct {
	c   *canonical.Canonical
	cfg *config.Config
}

// ---- model ----

type model struct {
	workspace string
	canonical *canonical.Canonical
	config    *config.Config
	adapters  []tools.Adapter

	screen screen
	w, h   int
	err    error

	// files screen
	files    []fileItem
	fileIdx  int
	preview  viewport.Model
	editing  bool
	editor   textarea.Model
	editBody string // original content before edit

	// tools screen
	toolItems []toolItem
	toolIdx   int

	// sync screen
	syncLines []string
	syncDone  bool
	logView   viewport.Model

	// divergence modal
	divResults []syncer.FileResult
	divChoices map[string]divChoice
	divIdx     int
	showDiv    bool

	// status from last syncer.Status() call
	statusMap map[string]syncer.FileStatus
}

// ---- helpers ----

func initialModel(workspace string, c *canonical.Canonical, cfg *config.Config, adapters []tools.Adapter) model {
	m := model{
		workspace:  workspace,
		canonical:  c,
		config:     cfg,
		adapters:   adapters,
		statusMap:  map[string]syncer.FileStatus{},
		divChoices: map[string]divChoice{},
	}
	m.files = buildFileItems(c)
	m.toolItems = buildToolItems(adapters, cfg, workspace)
	m.preview = viewport.New(80, 20)
	m.logView = viewport.New(80, 20)
	ta := textarea.New()
	ta.SetWidth(80)
	ta.SetHeight(20)
	ta.CharLimit = 0
	m.editor = ta
	return m
}

func buildFileItems(c *canonical.Canonical) []fileItem {
	items := []fileItem{{label: "AGENTS.md  (rules)", kind: kindRules}}
	for _, s := range c.Skills {
		items = append(items, fileItem{
			label: fmt.Sprintf("skills/%s/SKILL.md", s.Dir),
			kind:  kindSkill, skill: s,
		})
	}
	for _, a := range c.Agents {
		items = append(items, fileItem{
			label: fmt.Sprintf("agents/%s.md", a.Filename),
			kind:  kindAgent, agent: a,
		})
	}
	for _, cmd := range c.Commands {
		items = append(items, fileItem{
			label: fmt.Sprintf("commands/%s.md", cmd.Filename),
			kind:  kindCommand, command: cmd,
		})
	}
	return items
}

func buildToolItems(adapters []tools.Adapter, cfg *config.Config, workspace string) []toolItem {
	items := make([]toolItem, len(adapters))
	for i, a := range adapters {
		items[i] = toolItem{
			adapter: a,
			enabled: cfg.IsEnabled(a.Name()),
			install: a.Detect(workspace),
		}
	}
	return items
}

func (m *model) fileContent(idx int) string {
	if idx < 0 || idx >= len(m.files) {
		return ""
	}
	f := m.files[idx]
	switch f.kind {
	case kindRules:
		return m.canonical.Rules
	case kindSkill:
		return f.skill.Body
	case kindAgent:
		return f.agent.Body
	case kindCommand:
		return f.command.Body
	}
	return ""
}

func (m *model) fileStatusIcon(idx int) string {
	if idx < 0 || idx >= len(m.files) {
		return styleIconNew
	}
	f := m.files[idx]
	worst := -1 // sentinel: no matching paths found
	for path, status := range m.statusMap {
		if matchesFileItem(f, path) {
			if worst == -1 || int(status) > worst {
				worst = int(status)
			}
		}
	}
	if worst == -1 {
		return styleIconNew
	}
	switch syncer.FileStatus(worst) {
	case syncer.StatusSynced:
		return styleIconSynced
	case syncer.StatusDivergent:
		return styleIconDivergent
	case syncer.StatusMissing:
		return styleIconMissing
	default:
		return styleIconNew
	}
}

func matchesFileItem(f fileItem, path string) bool {
	switch f.kind {
	case kindRules:
		return strings.HasSuffix(path, "CLAUDE.md") ||
			strings.HasSuffix(path, "AGENTS.md") ||
			strings.HasSuffix(path, "GEMINI.md") ||
			strings.Contains(path, "general.mdc")
	case kindSkill:
		return strings.Contains(path, "skills/"+f.skill.Dir+"/")
	case kindAgent:
		return strings.Contains(path, "agents/"+f.agent.Filename+".md")
	case kindCommand:
		return strings.Contains(path, "commands/"+f.command.Filename+".md")
	}
	return false
}

// ---- tea commands ----

func checkStatusCmd(workspace string, c *canonical.Canonical, adapters []tools.Adapter, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		results, err := syncer.Status(workspace, c, adapters, cfg)
		if err != nil {
			return statusResultMsg{} // silently ignore on startup
		}
		return statusResultMsg{results: results}
	}
}

func runSyncCmd(workspace string, c *canonical.Canonical, adapters []tools.Adapter, cfg *config.Config, skip map[string]bool) tea.Cmd {
	return func() tea.Msg {
		result, err := syncer.RunSync(workspace, c, adapters, cfg, syncer.SyncOptions{Skip: skip})
		return syncDoneMsg{result: result, err: err}
	}
}

func reloadCmd(workspace string) tea.Cmd {
	return func() tea.Msg {
		c, err := canonical.Load(workspace)
		if err != nil {
			return nil
		}
		cfg, err := config.Load(workspace, tools.Names())
		if err != nil {
			return nil
		}
		return reloadMsg{c: c, cfg: cfg}
	}
}

// ---- Init ----

func (m model) Init() tea.Cmd {
	return checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config)
}

// ---- Update ----

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		panelH := m.h - 4 // tabs + footer
		m.preview = viewport.New(m.w/2-4, panelH)
		m.logView = viewport.New(m.w-4, panelH)
		m.editor.SetWidth(m.w/2 - 4)
		m.editor.SetHeight(panelH - 2)
		return m, nil

	case statusResultMsg:
		m.statusMap = make(map[string]syncer.FileStatus)
		for _, r := range msg.results {
			m.statusMap[r.Path] = r.Status
		}
		if len(msg.results) > 0 {
			m.divResults = nil
			for _, r := range msg.results {
				if r.Status == syncer.StatusDivergent || r.Status == syncer.StatusMissing {
					m.divResults = append(m.divResults, r)
				}
			}
		}
		return m, nil

	case syncDoneMsg:
		m.syncDone = true
		if msg.err != nil {
			m.syncLines = append(m.syncLines, fmt.Sprintf("Error: %v", msg.err))
		}
		if msg.result != nil {
			m.syncLines = append(m.syncLines, buildSyncLines(msg.result, m.adapters)...)
		}
		m.logView.SetContent(strings.Join(m.syncLines, "\n"))
		m.logView.GotoBottom()
		return m, checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config)

	case reloadMsg:
		m.canonical = msg.c
		m.config = msg.cfg
		m.files = buildFileItems(msg.c)
		m.toolItems = buildToolItems(m.adapters, msg.cfg, m.workspace)
		if m.fileIdx >= len(m.files) {
			m.fileIdx = 0
		}
		return m, checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config)
	}

	// When editing, forward keys to textarea
	if m.editing {
		return m.updateEditor(msg)
	}

	// When divergence modal is shown
	if m.showDiv {
		return m.updateDivModal(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "shift+tab", "right", "left", "l", "h", "1", "2", "3":
			switch msg.String() {
			case "tab", "right", "l":
				m.screen = screen((int(m.screen) + 1) % 3)
			case "shift+tab", "left", "h":
				m.screen = screen((int(m.screen) + 2) % 3)
			case "1":
				m.screen = screenFiles
			case "2":
				m.screen = screenTools
			case "3":
				m.screen = screenSync
			}
		case "j", "down":
			m = m.cursorDown()
		case "k", "up":
			m = m.cursorUp()
		case "e":
			if m.screen == screenFiles && len(m.files) > 0 {
				m = m.startEdit()
			}
		case "s":
			if m.screen == screenFiles || m.screen == screenSync {
				m.screen = screenSync
				var sc tea.Cmd
				m, sc = m.startSync()
				return m, sc
			}
		case "enter", " ":
			if m.screen == screenTools {
				m = m.toggleTool()
			}
		}
	}

	// Update preview content on file selection change
	m.updatePreview()

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	cmds = append(cmds, cmd)
	m.logView, cmd = m.logView.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			// save
			content := m.editor.Value()
			m.editing = false
			if err := m.saveCurrentFile(content); err != nil {
				m.err = err
				return m, nil
			}
			return m, reloadCmd(m.workspace)
		case "esc":
			m.editing = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m model) updateDivModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.showDiv = false
			return m, nil
		case "j", "down":
			if m.divIdx < len(m.divResults)-1 {
				m.divIdx++
			}
		case "k", "up":
			if m.divIdx > 0 {
				m.divIdx--
			}
		case "a":
			if m.divIdx < len(m.divResults) {
				m.divChoices[m.divResults[m.divIdx].Path] = choiceAdopt
			}
		case "o":
			if m.divIdx < len(m.divResults) {
				m.divChoices[m.divResults[m.divIdx].Path] = choiceOverwrite
			}
		case "d":
			if m.divIdx < len(m.divResults) {
				m.divChoices[m.divResults[m.divIdx].Path] = choiceDefer
			}
		case "enter":
			m.showDiv = false
			var cmd tea.Cmd
			m, cmd = m.applyDivChoices()
			return m, cmd
		}
	}
	return m, nil
}

func (m model) cursorDown() model {
	switch m.screen {
	case screenFiles:
		if m.fileIdx < len(m.files)-1 {
			m.fileIdx++
		}
	case screenTools:
		if m.toolIdx < len(m.toolItems)-1 {
			m.toolIdx++
		}
	}
	return m
}

func (m model) cursorUp() model {
	switch m.screen {
	case screenFiles:
		if m.fileIdx > 0 {
			m.fileIdx--
		}
	case screenTools:
		if m.toolIdx > 0 {
			m.toolIdx--
		}
	}
	return m
}

func (m *model) updatePreview() {
	if m.screen != screenFiles || len(m.files) == 0 {
		return
	}
	m.preview.SetContent(m.fileContent(m.fileIdx))
}

func (m model) startEdit() model {
	content := m.fileContent(m.fileIdx)
	m.editBody = content
	m.editor.SetValue(content)
	m.editor.Focus()
	m.editing = true
	return m
}

func (m model) toggleTool() model {
	if m.toolIdx >= len(m.toolItems) {
		return m
	}
	item := &m.toolItems[m.toolIdx]
	item.enabled = !item.enabled
	tc := m.config.Tools[item.adapter.Name()]
	tc.Enabled = item.enabled
	m.config.Tools[item.adapter.Name()] = tc
	_ = config.Save(m.workspace, m.config)
	return m
}

func (m model) startSync() (model, tea.Cmd) {
	m.syncLines = []string{"Starting sync…"}
	m.syncDone = false

	results, err := syncer.Status(m.workspace, m.canonical, m.adapters, m.config)
	if err != nil {
		return m, func() tea.Msg { return syncDoneMsg{err: err} }
	}

	var divs []syncer.FileResult
	for _, r := range results {
		if r.Status == syncer.StatusDivergent {
			divs = append(divs, r)
		}
	}
	if len(divs) > 0 {
		m.divResults = divs
		m.divChoices = map[string]divChoice{}
		m.divIdx = 0
		m.showDiv = true
		return m, nil
	}

	return m, runSyncCmd(m.workspace, m.canonical, m.adapters, m.config, nil)
}

func (m model) applyDivChoices() (model, tea.Cmd) {
	skip := map[string]bool{}
	for _, r := range m.divResults { // deterministic order
		switch m.divChoices[r.Path] {
		case choiceDefer, choiceNone: // unmarked = defer (safe default)
			skip[r.Path] = true
		case choiceAdopt:
			if err := syncer.AdoptExternal(m.workspace, r.Path); err != nil {
				skip[r.Path] = true
				m.syncLines = append(m.syncLines, fmt.Sprintf("  ✗ adopt %s: %v", r.Path, err))
			}
		case choiceOverwrite:
			// no-op: canonical will be written
		}
	}
	c, err := canonical.Load(m.workspace)
	if err != nil {
		return m, func() tea.Msg { return syncDoneMsg{err: err} }
	}
	m.canonical = c
	return m, runSyncCmd(m.workspace, c, m.adapters, m.config, skip)
}

func (m model) saveCurrentFile(content string) error {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return nil
	}
	f := m.files[m.fileIdx]
	switch f.kind {
	case kindRules:
		return canonical.SaveRules(m.workspace, content)
	case kindSkill:
		f.skill.Body = content
		return canonical.SaveSkill(m.workspace, f.skill)
	case kindAgent:
		f.agent.Body = content
		return canonical.SaveAgent(m.workspace, f.agent)
	case kindCommand:
		f.command.Body = content
		return canonical.SaveCommand(m.workspace, f.command)
	}
	return nil
}

// buildSyncLines formats the sync result as grouped lines: tool > concept > files.
func buildSyncLines(result *syncer.SyncResult, adapters []tools.Adapter) []string {
	conceptOrder := []tools.Concept{tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands}

	// Index written and skipped by toolName → concept → []path
	type entry struct {
		path     string
		deferred bool
	}
	grouped := map[string]map[tools.Concept][]entry{}
	for _, f := range result.Written {
		if grouped[f.ToolName] == nil {
			grouped[f.ToolName] = map[tools.Concept][]entry{}
		}
		grouped[f.ToolName][f.Concept] = append(grouped[f.ToolName][f.Concept], entry{path: f.Path})
	}
	for _, f := range result.Skipped {
		if grouped[f.ToolName] == nil {
			grouped[f.ToolName] = map[tools.Concept][]entry{}
		}
		grouped[f.ToolName][f.Concept] = append(grouped[f.ToolName][f.Concept], entry{path: f.Path, deferred: true})
	}

	var lines []string
	for _, a := range adapters {
		name := a.Name()
		byConceptEntries, ok := grouped[name]
		if !ok {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, styleSyncToolHeader.Render(name))
		for _, concept := range conceptOrder {
			entries, ok := byConceptEntries[concept]
			if !ok {
				continue
			}
			lines = append(lines, "  "+styleSyncConcept.Render(string(concept)))
			for _, e := range entries {
				if e.deferred {
					lines = append(lines, fmt.Sprintf("    – %s (deferred)", e.path))
				} else {
					lines = append(lines, fmt.Sprintf("    %s %s", styleBadgeOk, e.path))
				}
			}
		}
	}

	if len(result.Errors) > 0 {
		lines = append(lines, "")
		for _, e := range result.Errors {
			lines = append(lines, lipgloss.NewStyle().Foreground(colorDanger).Render(fmt.Sprintf("  ✗ %v", e)))
		}
	}

	count := len(result.Written)
	noun := "files"
	if count == 1 {
		noun = "file"
	}
	lines = append(lines, "")
	lines = append(lines, styleSyncSummary.Render(fmt.Sprintf("✓ Synced %d %s", count, noun)))
	return lines
}

// ---- View ----

func (m model) View() string {
	if m.w == 0 {
		return "Loading…"
	}

	tabs := m.renderTabs()
	var body string
	switch m.screen {
	case screenFiles:
		body = m.viewFiles()
	case screenTools:
		body = m.viewTools()
	case screenSync:
		body = m.viewSync()
	}
	footer := m.renderFooter()

	view := lipgloss.JoinVertical(lipgloss.Left, tabs, body, footer)
	if m.showDiv {
		view = m.overlayDivModal(view)
	}
	if m.err != nil {
		view = lipgloss.JoinVertical(lipgloss.Left, view,
			lipgloss.NewStyle().Foreground(colorDanger).Render(fmt.Sprintf("Error: %v", m.err)))
	}
	return view
}

func (m model) renderTabs() string {
	tabs := []string{"[1] Files", "[2] Tools", "[3] Sync"}
	rendered := make([]string, len(tabs))
	for i, t := range tabs {
		if screen(i) == m.screen {
			rendered[i] = styleTabActive.Render(t)
		} else {
			rendered[i] = styleTab.Render(t)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m model) renderFooter() string {
	var keys string
	switch {
	case m.editing:
		keys = "ctrl+s save • esc cancel"
	case m.showDiv:
		keys = "a adopt • o overwrite • d defer • enter apply • esc cancel"
	case m.screen == screenFiles:
		keys = "j/k move  •  e edit  •  s sync  •  h/← prev tab  •  l/→ next tab  •  q quit"
	case m.screen == screenTools:
		keys = "j/k move  •  space toggle  •  h/← prev tab  •  l/→ next tab  •  q quit"
	case m.screen == screenSync:
		keys = "s sync  •  h/← prev tab  •  l/→ next tab  •  q quit"
	}
	return styleFooter.Render(keys)
}

func (m model) viewFiles() string {
	leftW := m.w/3 - 2
	rightW := m.w - leftW - 7

	// left: file list
	var listLines []string
	for i, f := range m.files {
		icon := m.fileStatusIcon(i)
		var row string
		if i == m.fileIdx {
			row = styleCursorMark.Render("▍ ") + icon + "  " + styleSelected.Render(f.label)
		} else {
			row = "  " + icon + "  " + f.label
		}
		listLines = append(listLines, row)
	}

	listContent := strings.Join(listLines, "\n")
	leftPanel := stylePanelBorderActive.Width(leftW).Height(m.h - 4).Render(listContent)

	// right: preview or editor
	var rightPanel string
	if m.editing {
		rightPanel = stylePanelBorder.Width(rightW).Height(m.h - 4).Render(m.editor.View())
	} else {
		// compatibility badges
		var badges []string
		if len(m.files) > 0 {
			f := m.files[m.fileIdx]
			var concept tools.Concept
			switch f.kind {
			case kindRules:
				concept = tools.ConceptRules
			case kindSkill:
				concept = tools.ConceptSkills
			case kindAgent:
				concept = tools.ConceptAgents
			case kindCommand:
				concept = tools.ConceptCommands
			}
			for _, a := range m.adapters {
				compat := a.Supports(concept)
				switch {
				case compat.Supported && !compat.Deprecated:
					label := a.Name()
					if alias := a.Alias(concept); alias != "" {
						label = fmt.Sprintf("%s (%s)", a.Name(), alias)
					}
					badges = append(badges, fmt.Sprintf("%s %s", styleBadgeOk, label))
				case compat.Deprecated:
					badges = append(badges, fmt.Sprintf("%s %s — %s", styleBadgeWarn, a.Name(), compat.Reason))
				default:
					badges = append(badges, fmt.Sprintf("%s %s — %s", styleBadgeFail, a.Name(), compat.Reason))
				}
			}
		}

		m.preview.Width = rightW - 2
		m.preview.Height = m.h - 4 - len(badges) - 3
		m.preview.SetContent(m.fileContent(m.fileIdx))

		badgeStr := strings.Join(badges, "\n")
		rightContent := badgeStr + "\n\n" + m.preview.View()
		rightPanel = stylePanelBorder.Width(rightW).Height(m.h - 4).Render(rightContent)
	}

	return lipgloss.NewStyle().MarginLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel))
}

func (m model) viewTools() string {
	var lines []string
	lines = append(lines, styleTitle.Render("Agent Sync Targets"))
	lines = append(lines, "")

	for i, item := range m.toolItems {
		check := "[ ]"
		if item.enabled {
			check = lipgloss.NewStyle().Foreground(colorSuccess).Render("[x]")
		}

		installed := lipgloss.NewStyle().Foreground(colorMuted).Render("not installed")
		if item.install.Found {
			installed = lipgloss.NewStyle().Foreground(colorSuccess).Render("installed")
		}

		// concept badges
		concepts := []tools.Concept{tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands}
		conceptLabels := []string{"rules", "skills", "agents", "commands"}
		var conceptStr []string
		for ci, concept := range concepts {
			compat := item.adapter.Supports(concept)
			switch {
			case compat.Supported && !compat.Deprecated:
				conceptStr = append(conceptStr, styleBadgeOk+" "+conceptLabels[ci])
			case compat.Deprecated:
				badge := styleBadgeWarn + " " + conceptLabels[ci]
				if compat.Replacement != "" {
					badge += lipgloss.NewStyle().Foreground(colorMuted).Render(" (→ " + compat.Replacement + ")")
				}
				conceptStr = append(conceptStr, badge)
			default:
				conceptStr = append(conceptStr, styleBadgeFail+" "+conceptLabels[ci])
			}
		}

		var row string
		if i == m.toolIdx {
			name := styleSelected.Render(fmt.Sprintf("%-14s", item.adapter.Name()))
			row = styleCursorMark.Render("▍ ") + check + "  " + name + "  " + installed + "  " + strings.Join(conceptStr, "  ")
		} else {
			row = "  " + fmt.Sprintf("%s  %-14s  %-15s  %s", check, item.adapter.Name(), installed, strings.Join(conceptStr, "  "))
		}
		lines = append(lines, row)
	}

	content := strings.Join(lines, "\n")
	return stylePanelBorder.Width(m.w - 5).Height(m.h - 4).MarginLeft(1).Render(content)
}

func (m model) viewSync() string {
	header := styleTitle.Render("Sync")
	if !m.syncDone && len(m.syncLines) == 0 {
		header += "\n\nPress [s] to sync canonical → all enabled tool folders."
	}
	m.logView.Width = m.w - 7
	m.logView.Height = m.h - 8
	m.logView.SetContent(strings.Join(m.syncLines, "\n"))
	content := header + "\n\n" + m.logView.View()
	return stylePanelBorderInset.Width(m.w - 5).Height(m.h - 4).Render(content)
}

func (m model) overlayDivModal(base string) string {
	choiceLabels := map[divChoice]string{
		choiceNone:      "  (none) ",
		choiceAdopt:     " [adopt] ",
		choiceOverwrite: " [overwrite] ",
		choiceDefer:     " [defer] ",
	}

	var lines []string
	lines = append(lines,
		lipgloss.NewStyle().Bold(true).Foreground(colorWarn).Render("⚠  Divergent files detected"),
		"",
		"Files edited outside agentsync. Choose action per file:",
		"  a = adopt  o = overwrite  d = defer  (unmarked = defer)",
		"  If multiple rules files are adopted, the last one in this list wins.",
		"",
	)

	for i, r := range m.divResults {
		choice := m.divChoices[r.Path]
		choiceStr := choiceLabels[choice]
		icon := styleIconDivergent
		if r.Status == syncer.StatusMissing {
			icon = styleIconMissing
		}
		var row string
		if i == m.divIdx {
			path := styleSelected.Render(fmt.Sprintf("%-40s", r.Path))
			row = styleCursorMark.Render("▍ ") + icon + "  " + path + "  " + choiceStr + "  [" + r.ToolName + "]"
		} else {
			row = "  " + fmt.Sprintf("%s  %-40s  %s  [%s]", icon, r.Path, choiceStr, r.ToolName)
		}
		lines = append(lines, row)
	}

	lines = append(lines, "", "Press [enter] to apply choices and continue sync.")

	modal := styleModalBorder.
		Width(m.w - 8).
		Render(strings.Join(lines, "\n"))

	// center the modal vertically
	topPad := (m.h - strings.Count(modal, "\n") - 4) / 2
	if topPad < 0 {
		topPad = 0
	}
	padding := strings.Repeat("\n", topPad)
	return padding + modal
}

// ---- Run ----

// Run starts the agentsync TUI.
func Run(workspace string, adapters []tools.Adapter) error {
	c, err := canonical.Load(workspace)
	if err != nil {
		return fmt.Errorf("load canonical: %w", err)
	}
	cfg, err := config.Load(workspace, tools.Names())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	m := initialModel(workspace, c, cfg, adapters)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
