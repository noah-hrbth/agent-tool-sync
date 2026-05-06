package tui

import (
	"fmt"
	"os"
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
	kindAgentsMD fileKind = iota
	kindRule
	kindSkill
	kindAgent
	kindCommand
)

type fileItem struct {
	label   string
	kind    fileKind
	rule    *canonical.Rule
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

// scopeSnapshot bundles per-scope state that the TUI swaps in/out on toggle.
// The active scope's snapshot is mirrored into the model's flat fields
// (workspace/canonical/config) for backward compatibility with existing code.
type scopeSnapshot struct {
	base        string
	canonical   *canonical.Canonical
	config      *config.Config
	scope       tools.Scope
	loaded      bool   // false until first activation
	initialized bool   // false when .agentsync/ doesn't exist for this scope
	loadErr     error
}

// ---- model ----

type model struct {
	workspace string
	canonical *canonical.Canonical
	config    *config.Config
	adapters  []tools.Adapter

	scope       tools.Scope
	initialized bool           // false when active scope's .agentsync/ doesn't exist
	inactive    *scopeSnapshot // the other scope, lazy-loaded on first toggle

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

func initialModel(workspace string, scope tools.Scope, c *canonical.Canonical, cfg *config.Config, adapters []tools.Adapter) model {
	m := model{
		workspace:   workspace,
		canonical:   c,
		config:      cfg,
		adapters:    adapters,
		scope:       scope,
		initialized: true,
		statusMap:   map[string]syncer.FileStatus{},
		divChoices:  map[string]divChoice{},
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
	items := []fileItem{{label: "AGENTS.md", kind: kindAgentsMD}}
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
	for _, r := range c.Rules {
		items = append(items, fileItem{
			label: fmt.Sprintf("rules/%s.md", r.Filename),
			kind:  kindRule, rule: r,
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
	case kindAgentsMD:
		return m.canonical.AgentsMD
	case kindRule:
		out, err := canonical.RenderRule(f.rule)
		if err != nil {
			return f.rule.Body
		}
		return out
	case kindSkill:
		out, err := canonical.RenderSkill(f.skill)
		if err != nil {
			return f.skill.Body
		}
		return out
	case kindAgent:
		out, err := canonical.RenderAgent(f.agent)
		if err != nil {
			return f.agent.Body
		}
		return out
	case kindCommand:
		out, err := canonical.RenderCommand(f.command)
		if err != nil {
			return f.command.Body
		}
		return out
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
	case kindAgentsMD:
		// Cursor's general.mdc is the rendered AGENTS.md catch-all — include it here.
		if path == ".cursor/rules/general.mdc" {
			return true
		}
		// Root memory files: match by filename, but not paths inside a /rules/ dir
		// (those are per-rule files, matched by kindRule below).
		if strings.Contains(path, "/rules/") {
			return false
		}
		return strings.HasSuffix(path, "CLAUDE.md") ||
			strings.HasSuffix(path, "AGENTS.md") ||
			strings.HasSuffix(path, "GEMINI.md")
	case kindRule:
		if !strings.Contains(path, "/rules/") {
			return false
		}
		base := path[strings.LastIndex(path, "/")+1:]
		// Strip extension (.md or .mdc) to get the filename slug.
		slug := strings.TrimSuffix(strings.TrimSuffix(base, ".mdc"), ".md")
		return slug == f.rule.Filename
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

func checkStatusCmd(workspace string, c *canonical.Canonical, adapters []tools.Adapter, cfg *config.Config, scope tools.Scope) tea.Cmd {
	return func() tea.Msg {
		results, err := syncer.Status(workspace, c, adapters, cfg, scope)
		if err != nil {
			return statusResultMsg{} // silently ignore on startup
		}
		return statusResultMsg{results: results}
	}
}

func runSyncCmd(workspace string, c *canonical.Canonical, adapters []tools.Adapter, cfg *config.Config, scope tools.Scope, skip map[string]bool) tea.Cmd {
	return func() tea.Msg {
		result, err := syncer.RunSync(workspace, c, adapters, cfg, scope, syncer.SyncOptions{Skip: skip})
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
	return checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope)
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
		return m, checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope)

	case reloadMsg:
		m.canonical = msg.c
		m.config = msg.cfg
		m.files = buildFileItems(msg.c)
		m.toolItems = buildToolItems(m.adapters, msg.cfg, m.workspace)
		if m.fileIdx >= len(m.files) {
			m.fileIdx = 0
		}
		return m, checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope)
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
			return m, nil
		case "k", "up":
			m = m.cursorUp()
			return m, nil
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
		case "g":
			var cmd tea.Cmd
			m, cmd = m.toggleScope()
			return m, cmd
		}
	}

	// Update preview content on file selection change
	m.updatePreview()

	var cmd tea.Cmd
	switch m.screen {
	case screenFiles:
		m.preview, cmd = m.preview.Update(msg)
	case screenSync:
		m.logView, cmd = m.logView.Update(msg)
	}
	return m, cmd
}

func (m model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			content := m.editor.Value()
			if err := m.saveCurrentFile(content); err != nil {
				m.err = err
				return m, nil
			}
			m.editing = false
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

// toggleScope swaps between project and user scope. The inactive scope is
// lazy-loaded the first time it's activated. If the inactive scope's
// .agentsync/ doesn't exist, the model still toggles but renders an empty
// state with init guidance.
func (m model) toggleScope() (model, tea.Cmd) {
	// Bank the current (now-leaving) state into m.inactive.
	leaving := scopeSnapshot{
		base:        m.workspace,
		canonical:   m.canonical,
		config:      m.config,
		scope:       m.scope,
		loaded:      true,
		initialized: m.initialized,
	}

	// Resolve or load the snapshot we're switching to.
	var entering *scopeSnapshot
	if m.inactive != nil && m.inactive.loaded {
		entering = m.inactive
	} else {
		other := otherScope(m.scope)
		base, err := scopeBase(other)
		if err != nil {
			m.err = err
			return m, nil
		}
		s := loadScopeSnapshot(base, other)
		entering = &s
	}

	if entering.loadErr != nil {
		m.err = entering.loadErr
		return m, nil
	}

	m.scope = entering.scope
	m.workspace = entering.base
	m.canonical = entering.canonical
	m.config = entering.config
	m.initialized = entering.initialized
	m.files = buildFileItems(entering.canonical)
	m.toolItems = buildToolItems(m.adapters, entering.config, entering.base)
	m.fileIdx = 0
	m.statusMap = map[string]syncer.FileStatus{}
	m.divResults = nil
	m.inactive = &leaving

	if !entering.initialized {
		// No canonical at this scope yet — clear status to avoid stale results.
		return m, nil
	}
	return m, checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope)
}

func otherScope(s tools.Scope) tools.Scope {
	if s == tools.ScopeUser {
		return tools.ScopeProject
	}
	return tools.ScopeUser
}

// scopeBase returns the canonical-root directory for a scope. For ScopeUser this
// is the user's home dir. For ScopeProject the user-invoked TUI's cwd is used,
// which the model already holds — so this helper only resolves user scope.
// (Toggling from user to project requires the original project base; that path
// lives in m.inactive after first toggle.)
func scopeBase(s tools.Scope) (string, error) {
	if s == tools.ScopeUser {
		return os.UserHomeDir()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

// loadScopeSnapshot reads canonical + config from base/.agentsync/. If the
// directory doesn't exist it returns initialized=false with an empty canonical
// so the TUI can render an empty state without erroring.
func loadScopeSnapshot(base string, scope tools.Scope) scopeSnapshot {
	s := scopeSnapshot{base: base, scope: scope, loaded: true}
	if _, err := os.Stat(base + "/.agentsync"); os.IsNotExist(err) {
		s.canonical = &canonical.Canonical{Workspace: base}
		s.config = config.Default(tools.Names())
		s.initialized = false
		return s
	}
	c, err := canonical.Load(base)
	if err != nil {
		s.loadErr = err
		return s
	}
	cfg, err := config.Load(base, tools.Names())
	if err != nil {
		s.loadErr = err
		return s
	}
	s.canonical = c
	s.config = cfg
	s.initialized = true
	return s
}

func (m model) startSync() (model, tea.Cmd) {
	m.syncLines = []string{"Starting sync…"}
	m.syncDone = false

	results, err := syncer.Status(m.workspace, m.canonical, m.adapters, m.config, m.scope)
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

	return m, runSyncCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope, nil)
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
	return m, runSyncCmd(m.workspace, c, m.adapters, m.config, m.scope, skip)
}

func (m model) saveCurrentFile(content string) error {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return nil
	}
	f := m.files[m.fileIdx]
	switch f.kind {
	case kindAgentsMD:
		return canonical.SaveAgentsMD(m.workspace, content)
	case kindRule:
		if err := canonical.ParseRule(content, f.rule); err != nil {
			return err
		}
		return canonical.SaveRule(m.workspace, f.rule)
	case kindSkill:
		if err := canonical.ParseSkill(content, f.skill); err != nil {
			return err
		}
		return canonical.SaveSkill(m.workspace, f.skill)
	case kindAgent:
		if err := canonical.ParseAgent(content, f.agent); err != nil {
			return err
		}
		return canonical.SaveAgent(m.workspace, f.agent)
	case kindCommand:
		if err := canonical.ParseCommand(content, f.command); err != nil {
			return err
		}
		return canonical.SaveCommand(m.workspace, f.command)
	}
	return nil
}

// buildSyncLines formats the sync result as grouped lines: tool > concept > files.
// Root memory files are shown under "AGENTS.md"; rules-dir files under "rules".
// Cursor has no "AGENTS.md" subgroup — its general.mdc lives in /rules/.
func buildSyncLines(result *syncer.SyncResult, adapters []tools.Adapter) []string {
	displayOrder := []string{"AGENTS.md", string(tools.ConceptSkills), string(tools.ConceptAgents), string(tools.ConceptCommands), string(tools.ConceptRules)}

	type entry struct {
		path     string
		deferred bool
	}
	// group: toolName → displayBucket → []entry
	grouped := map[string]map[string][]entry{}

	addEntry := func(toolName, bucket, path string, deferred bool) {
		if grouped[toolName] == nil {
			grouped[toolName] = map[string][]entry{}
		}
		grouped[toolName][bucket] = append(grouped[toolName][bucket], entry{path: path, deferred: deferred})
	}

	for _, f := range result.Written {
		addEntry(f.ToolName, displayConcept(f.Path, f.Concept), f.Path, false)
	}
	for _, f := range result.Skipped {
		addEntry(f.ToolName, displayConcept(f.Path, f.Concept), f.Path, true)
	}

	var lines []string
	for _, a := range adapters {
		name := a.Name()
		byBucket, ok := grouped[name]
		if !ok {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, styleSyncToolHeader.Render(name))
		for _, bucket := range displayOrder {
			entries, ok := byBucket[bucket]
			if !ok {
				continue
			}
			lines = append(lines, "  "+styleSyncConcept.Render(bucket))
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

// displayConcept classifies a synced file path into its display bucket.
// Root memory files (no /rules/ in path) → "AGENTS.md".
// Everything else → the raw concept string.
func displayConcept(path string, concept tools.Concept) string {
	if concept == tools.ConceptRules && !strings.Contains(path, "/rules/") {
		return "AGENTS.md"
	}
	return string(concept)
}

// ruleAppendNotice returns a parenthetical label for tools that do not support
// per-file rules and instead append the rule body to their root memory file.
// Returns "" for tools that write individual rule files (Claude Code, Cursor).
func ruleAppendNotice(adapterName string) string {
	switch adapterName {
	case "Gemini CLI":
		return "appended to GEMINI.md"
	case "OpenCode":
		return "appended to AGENTS.md"
	case "Codex CLI":
		return "appended to AGENTS.md"
	case "Zed":
		return "appended to .rules"
	case "JetBrains Junie":
		return "appended to AGENTS.md"
	default:
		return ""
	}
}

func scopeTitle(s tools.Scope) string {
	if s == tools.ScopeUser {
		return "User"
	}
	return "Project"
}

func (m model) viewUninitialized() string {
	cmdHint := "agentsync init"
	if m.scope == tools.ScopeUser {
		cmdHint = "agentsync init --global"
	}
	lines := []string{
		styleTitle.Render(fmt.Sprintf("%s scope is not initialized", scopeTitle(m.scope))),
		"",
		fmt.Sprintf("No .agentsync/ found at %s", m.workspace),
		"",
		fmt.Sprintf("Run: %s", lipgloss.NewStyle().Foreground(colorSuccess).Bold(true).Render(cmdHint)),
		"",
		"Press [g] to switch back to the other scope.",
	}
	return stylePanelBorder.Width(m.w - 5).Height(m.h - 4).MarginLeft(1).Render(strings.Join(lines, "\n"))
}

// ---- View ----

func (m model) View() string {
	if m.w == 0 {
		return "Loading…"
	}

	tabs := m.renderTabs()
	var body string
	if !m.initialized {
		body = m.viewUninitialized()
	} else {
		switch m.screen {
		case screenFiles:
			body = m.viewFiles()
		case screenTools:
			body = m.viewTools()
		case screenSync:
			body = m.viewSync()
		}
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
	scopeLabel := fmt.Sprintf("  scope: %s  [g]", m.scope)
	return lipgloss.JoinHorizontal(lipgloss.Top, append(rendered, lipgloss.NewStyle().Foreground(colorMuted).Render(scopeLabel))...)
}

func (m model) renderFooter() string {
	var keys string
	switch {
	case m.editing:
		keys = "ctrl+s save • esc cancel"
	case m.showDiv:
		keys = "a adopt • o overwrite • d defer • enter apply • esc cancel"
	case m.screen == screenFiles:
		keys = "j/k move  •  u/d scroll  •  e edit  •  s sync  •  g scope  •  h/← l/→ tabs  •  q quit"
	case m.screen == screenTools:
		keys = "j/k move  •  space toggle  •  g scope  •  h/← l/→ tabs  •  q quit"
	case m.screen == screenSync:
		keys = "s sync  •  g scope  •  h/← l/→ tabs  •  q quit"
	}
	return styleFooter.Render(keys)
}

// groupHeader returns the section header label for the given fileKind, or an
// empty string if this kind is the same group as the previous item.
func groupHeader(prev, cur fileKind) string {
	if cur == prev {
		return ""
	}
	switch cur {
	case kindAgentsMD:
		return "── AGENTS.md ──"
	case kindSkill:
		return "── Skills ──"
	case kindAgent:
		return "── Subagents ──"
	case kindCommand:
		return "── Commands ──"
	case kindRule:
		return "── Rules ──"
	}
	return ""
}

func (m model) viewFiles() string {
	leftW := m.w/3 - 2
	rightW := m.w - leftW - 7

	// left: file list with section headers
	var listLines []string
	prevKind := fileKind(-1)
	for i, f := range m.files {
		if hdr := groupHeader(prevKind, f.kind); hdr != "" {
			if prevKind != fileKind(-1) {
				listLines = append(listLines, "")
			}
			listLines = append(listLines, "  "+styleFileGroupHeader.Render(hdr))
		}
		prevKind = f.kind

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
			case kindAgentsMD, kindRule:
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
					if f.kind == kindRule {
						if notice := ruleAppendNotice(a.Name()); notice != "" {
							label = fmt.Sprintf("%s (%s)", a.Name(), notice)
						}
					} else {
						if alias := a.Alias(concept); alias != "" {
							label = fmt.Sprintf("%s (%s)", a.Name(), alias)
						}
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
		if note := item.adapter.Notice(); note != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(colorMuted).Render("       ↳ "+note))
		}
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
	m.logView.Height = m.h - 9
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

// Run starts the agentsync TUI rooted at workspace with the given initial scope.
// The other scope is lazy-loaded on first toggle. If the active scope's
// .agentsync/ doesn't exist, the TUI launches with an empty-state banner
// instead of failing.
func Run(workspace string, scope tools.Scope, adapters []tools.Adapter) error {
	snap := loadScopeSnapshot(workspace, scope)
	if snap.loadErr != nil {
		return fmt.Errorf("load %s scope: %w", scope, snap.loadErr)
	}

	m := initialModel(workspace, scope, snap.canonical, snap.config, adapters)
	m.initialized = snap.initialized
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
