package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/gitignore"
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
	label       string
	kind        fileKind
	placeholder bool // true → no underlying entity; group is empty
	rule        *canonical.Rule
	skill       *canonical.Skill
	agent       *canonical.Agent
	command     *canonical.Command
}

// pendingSel records a not-yet-loaded selection so a reloadMsg can position
// the cursor on a freshly created file and auto-enter edit mode.
type pendingSel struct {
	kind fileKind
	slug string
}

type toolItem struct {
	adapter tools.Tool
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
	loaded      bool // false until first activation
	initialized bool // false when .agentsync/ doesn't exist for this scope
	loadErr     error
}

// tabRange is a half-open [start, end) X-coordinate span used by mouse hit-testing.
type tabRange struct{ start, end int }

// layoutMetrics captures the geometry View() draws so Update() can route mouse
// events to the same regions. All Y coordinates are 0-based screen rows.
type layoutMetrics struct {
	tabRowY     int
	tabXRanges  []tabRange // [1] Files, [2] Tools, [3] Sync (in that order)
	scopeXRange tabRange   // "  scope: <s>  [g]"
	bodyTopY    int        // first row of the body panel
	bodyBottomY int        // first row past the body panel (exclusive)

	// Files screen
	filesPanelTopY    int
	filesPanelBottomY int
	filesLeftPanelX   int
	filesLeftPanelW   int
	filesRightPanelX  int
	filesRightPanelW  int
	filesListInnerY0  int // Y where listLines[0] is drawn (inside left panel)

	// Tools screen
	toolsPanelX int
	toolsPanelW int

	// Sync screen
	syncPanelX int
	syncPanelW int
}

// ---- model ----

type model struct {
	workspace string
	canonical *canonical.Canonical
	config    *config.Config
	adapters  []tools.Tool

	scope       tools.Scope
	initialized bool           // false when active scope's .agentsync/ doesn't exist
	inactive    *scopeSnapshot // the other scope, lazy-loaded on first toggle

	screen screen
	w, h   int
	err    error

	// files screen
	files    []fileItem
	fileIdx  int
	fileList viewport.Model // wraps the left list
	preview  viewport.Model
	editing  bool
	editor   textarea.Model
	editBody string // original content before edit

	// new-file prompt (textinput modal)
	input         textinput.Model
	inputting     bool
	inputKind     fileKind
	inputErr      string
	pendingSelect *pendingSel

	// delete confirmation modal
	confirmingDelete bool
	deleteTarget     int    // index into m.files
	deleteErr        string // surfaced if removal fails

	// tools screen
	toolItems    []toolItem
	toolIdx      int
	toolList     viewport.Model // wraps the rows (excluding title)
	showToolInfo bool

	// sync screen
	syncLines []string
	syncDone  bool
	logView   viewport.Model

	// divergence modal
	divResults []syncer.FileResult
	divChoices map[string]divChoice
	divIdx     int
	showDiv    bool

	// first-sync gitignore prompt modal
	showGitignorePrompt bool

	// status from last syncer.Status() call
	statusMap map[string]syncer.FileStatus

	// transient one-line status (e.g. "copied to clipboard"); cleared on next keystroke
	flash string
}

// clipboardWrite is the seam used by the copy keybinding. Tests override it to
// avoid shelling out to pbcopy/xclip.
var clipboardWrite = clipboard.WriteAll

// ---- helpers ----

func initialModel(workspace string, scope tools.Scope, c *canonical.Canonical, cfg *config.Config, adapters []tools.Tool) model {
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
	m.preview.KeyMap = vimViewportKeyMap()
	m.logView = viewport.New(80, 20)
	m.logView.KeyMap = vimViewportKeyMap()
	// fileList and toolList route ctrl+d/u manually (see Update); strip default
	// viewport keybindings so they don't hijack j/k/g/G/u/d.
	m.fileList = viewport.New(80, 20)
	m.fileList.MouseWheelEnabled = true
	m.fileList.KeyMap = viewport.KeyMap{}
	m.toolList = viewport.New(80, 20)
	m.toolList.MouseWheelEnabled = true
	m.toolList.KeyMap = viewport.KeyMap{}
	ta := textarea.New()
	ta.SetWidth(80)
	ta.SetHeight(20)
	ta.CharLimit = 0
	m.editor = ta
	ti := textinput.New()
	ti.CharLimit = 64
	ti.Width = 40
	m.input = ti
	return m
}

func buildFileItems(c *canonical.Canonical) []fileItem {
	items := []fileItem{{label: "AGENTS.md", kind: kindAgentsMD}}

	if len(c.Skills) == 0 {
		items = append(items, fileItem{label: "(no skills yet — press n to create)", kind: kindSkill, placeholder: true})
	} else {
		for _, s := range c.Skills {
			items = append(items, fileItem{
				label: fmt.Sprintf("skills/%s/SKILL.md", s.Dir),
				kind:  kindSkill, skill: s,
			})
		}
	}

	if len(c.Agents) == 0 {
		items = append(items, fileItem{label: "(no subagents yet — press n to create)", kind: kindAgent, placeholder: true})
	} else {
		for _, a := range c.Agents {
			items = append(items, fileItem{
				label: fmt.Sprintf("agents/%s.md", a.Filename),
				kind:  kindAgent, agent: a,
			})
		}
	}

	if len(c.Commands) == 0 {
		items = append(items, fileItem{label: "(no commands yet — press n to create)", kind: kindCommand, placeholder: true})
	} else {
		for _, cmd := range c.Commands {
			items = append(items, fileItem{
				label: fmt.Sprintf("commands/%s.md", cmd.Filename),
				kind:  kindCommand, command: cmd,
			})
		}
	}

	if len(c.Rules) == 0 {
		items = append(items, fileItem{label: "(no rules yet — press n to create)", kind: kindRule, placeholder: true})
	} else {
		for _, r := range c.Rules {
			items = append(items, fileItem{
				label: fmt.Sprintf("rules/%s.md", r.Filename),
				kind:  kindRule, rule: r,
			})
		}
	}

	return items
}

func buildToolItems(adapters []tools.Tool, cfg *config.Config, workspace string) []toolItem {
	items := make([]toolItem, len(adapters))
	for i, a := range adapters {
		items[i] = toolItem{
			adapter: a,
			enabled: cfg.IsEnabled(a.Meta.Name),
			install: a.Meta.Detect(workspace),
		}
	}
	return items
}

// fileContent returns the raw on-disk content for the file at idx.
// Used by the editor — no cosmetic spacing.
func (m *model) fileContent(idx int) string {
	if idx < 0 || idx >= len(m.files) {
		return ""
	}
	f := m.files[idx]
	if f.placeholder {
		return placeholderHint(f.kind)
	}
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

// previewContent is fileContent with cosmetic spacing applied for the preview pane.
func (m *model) previewContent(idx int) string {
	s := m.fileContent(idx)
	if idx < 0 || idx >= len(m.files) || m.files[idx].placeholder {
		return s
	}
	if m.files[idx].kind == kindAgentsMD {
		return s
	}
	return spaceFrontmatter(s)
}

// spaceFrontmatter adds a blank line between the closing "---" fence and the
// body for nicer preview rendering. Display-only — disk format is unchanged.
func spaceFrontmatter(s string) string {
	const fence = "\n---\n"
	idx := strings.Index(s, fence)
	if idx < 0 {
		return s
	}
	bodyStart := idx + len(fence)
	if bodyStart >= len(s) {
		return s
	}
	if s[bodyStart] == '\n' {
		return s
	}
	return s[:bodyStart] + "\n" + s[bodyStart:]
}

// vimViewportKeyMap returns the viewport keybindings without plain "u"/"d"
// half-page bindings, freeing those keys for our own commands. ctrl+u/ctrl+d
// keep working for half-page scroll, matching vim convention.
func vimViewportKeyMap() viewport.KeyMap {
	km := viewport.DefaultKeyMap()
	km.HalfPageUp = key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "½ page up"),
	)
	km.HalfPageDown = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "½ page down"),
	)
	return km
}

// placeholderHint returns the preview text shown when an empty-group
// placeholder row is selected.
func placeholderHint(k fileKind) string {
	switch k {
	case kindSkill:
		return "No skills yet.\n\nPress [n] to create a new skill folder.\nThe folder name you choose becomes the skill's dir; SKILL.md is created automatically inside it."
	case kindAgent:
		return "No subagents yet.\n\nPress [n] to create a new subagent file."
	case kindCommand:
		return "No commands yet.\n\nPress [n] to create a new slash command file."
	case kindRule:
		return "No rules yet.\n\nPress [n] to create a new rule file."
	}
	return ""
}

func (m *model) fileStatusIcon(idx int) string {
	if idx < 0 || idx >= len(m.files) {
		return styleIconNew
	}
	f := m.files[idx]
	if f.placeholder {
		return styleIconPlaceholder
	}
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

func checkStatusCmd(workspace string, c *canonical.Canonical, adapters []tools.Tool, cfg *config.Config, scope tools.Scope) tea.Cmd {
	return func() tea.Msg {
		results, err := syncer.Status(workspace, c, adapters, cfg, scope)
		if err != nil {
			return statusResultMsg{} // silently ignore on startup
		}
		return statusResultMsg{results: results}
	}
}

func runSyncCmd(workspace string, c *canonical.Canonical, adapters []tools.Tool, cfg *config.Config, scope tools.Scope, skip map[string]bool) tea.Cmd {
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
		m.resizeViewports()
		m.refreshFileList()
		m.refreshToolList()
		m.updatePreview()
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
		// Status icons (●/▲/○/+) live inside the file list rows; rerender so
		// the viewport reflects the new state.
		m.refreshFileList()
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
		// If a new file was just created, jump to it and open the editor.
		if m.pendingSelect != nil {
			ps := *m.pendingSelect
			m.pendingSelect = nil
			if idx := findFileIndex(m.files, ps.kind, ps.slug); idx >= 0 {
				m.fileIdx = idx
				m = m.startEdit()
			}
		}
		m.refreshFileList()
		m.refreshToolList()
		m.updatePreview()
		return m, checkStatusCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope)

	case tea.MouseMsg:
		// Mouse events are dropped while any modal or the editor is open. The
		// trailing viewport.Update forwarder below would otherwise scroll the
		// viewport behind the modal on every wheel tick (viewport.Update reacts
		// to MouseMsg without bounds-checking X/Y).
		if m.editing || m.inputting || m.confirmingDelete || m.showDiv || m.showToolInfo || m.showGitignorePrompt {
			return m, nil
		}
		return m.handleMouse(msg)
	}

	// When editing, forward keys to textarea
	if m.editing {
		return m.updateEditor(msg)
	}

	// When the new-file textinput modal is open, forward keys to it.
	if m.inputting {
		return m.updateInput(msg)
	}

	// When the delete confirmation modal is open, forward keys to it.
	if m.confirmingDelete {
		return m.updateDeleteConfirm(msg)
	}

	// When divergence modal is shown
	if m.showDiv {
		return m.updateDivModal(msg)
	}

	// When tool info modal is open
	if m.showToolInfo {
		return m.updateToolInfo(msg)
	}

	// When first-sync gitignore prompt is open
	if m.showGitignorePrompt {
		return m.updateGitignorePrompt(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.flash = ""
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "c":
			if m.screen == screenFiles && len(m.files) > 0 && !m.files[m.fileIdx].placeholder {
				if err := clipboardWrite(m.fileContent(m.fileIdx)); err != nil {
					m.flash = "✗ copy failed: " + err.Error()
				} else {
					m.flash = "✓ copied " + m.files[m.fileIdx].label + " to clipboard"
				}
				return m, nil
			}
		case "tab", "shift+tab", "right", "left", "l", "h", "1", "2", "3":
			switch msg.String() {
			case "tab", "right", "l":
				m.setScreen(screen((int(m.screen) + 1) % 3))
			case "shift+tab", "left", "h":
				m.setScreen(screen((int(m.screen) + 2) % 3))
			case "1":
				m.setScreen(screenFiles)
			case "2":
				m.setScreen(screenTools)
			case "3":
				m.setScreen(screenSync)
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
		case "n":
			if m.screen == screenFiles && len(m.files) > 0 {
				m = m.startNewFile()
			}
		case "d":
			if m.screen == screenFiles && len(m.files) > 0 {
				m = m.startDeleteFile()
			}
		case "s":
			if m.screen == screenFiles || m.screen == screenSync {
				m.screen = screenSync
				var sc tea.Cmd
				m, sc = m.startSync()
				return m, sc
			}
		case " ":
			if m.screen == screenTools {
				m = m.toggleTool()
			}
		case "enter":
			if m.screen == screenTools && len(m.toolItems) > 0 {
				m.showToolInfo = true
			}
		case "g":
			var cmd tea.Cmd
			m, cmd = m.toggleScope()
			return m, cmd
		case "ctrl+d":
			switch m.screen {
			case screenFiles:
				m.preview.HalfPageDown()
			case screenTools:
				m.toolList.HalfPageDown()
			case screenSync:
				m.logView.HalfPageDown()
			}
			return m, nil
		case "ctrl+u":
			switch m.screen {
			case screenFiles:
				m.preview.HalfPageUp()
			case screenTools:
				m.toolList.HalfPageUp()
			case screenSync:
				m.logView.HalfPageUp()
			}
			return m, nil
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

func (m model) updateToolInfo(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "enter", "q":
			m.showToolInfo = false
		}
	}
	return m, nil
}

// updateGitignorePrompt routes the first-sync gitignore modal: 'a' applies, 's'
// skips, 'esc' dismisses without persisting (modal re-appears on next sync).
// Apply/Skip both close the modal, persist config, and dispatch the sync.
func (m model) updateGitignorePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc":
		m.showGitignorePrompt = false
		return m, nil
	case "a", "A":
		if err := gitignore.Update(m.workspace, gitignore.Compute(m.adapters)); err != nil {
			m.err = err
			m.showGitignorePrompt = false
			return m, nil
		}
		m.config.Gitignore = config.GitignoreConfig{Manage: true, Prompted: true}
		if err := config.Save(m.workspace, m.config); err != nil {
			m.err = err
		}
		m.showGitignorePrompt = false
		return m, runSyncCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope, nil)
	case "s", "S":
		if err := gitignore.Remove(m.workspace); err != nil {
			m.err = err
			m.showGitignorePrompt = false
			return m, nil
		}
		m.config.Gitignore = config.GitignoreConfig{Manage: false, Prompted: true}
		if err := config.Save(m.workspace, m.config); err != nil {
			m.err = err
		}
		m.showGitignorePrompt = false
		return m, runSyncCmd(m.workspace, m.canonical, m.adapters, m.config, m.scope, nil)
	}
	return m, nil
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
		m.followFileCursor()
		m.refreshFileList()
	case screenTools:
		if m.toolIdx < len(m.toolItems)-1 {
			m.toolIdx++
		}
		m.followToolCursor()
		m.refreshToolList()
	}
	return m
}

func (m model) cursorUp() model {
	switch m.screen {
	case screenFiles:
		if m.fileIdx > 0 {
			m.fileIdx--
		}
		m.followFileCursor()
		m.refreshFileList()
	case screenTools:
		if m.toolIdx > 0 {
			m.toolIdx--
		}
		m.followToolCursor()
		m.refreshToolList()
	}
	return m
}

// updatePreview renders the currently-selected file's content into the preview
// viewport. Wraps lines at the viewport's current width so HalfPageDown/Up
// advance over the same visual rows that View() draws.
func (m *model) updatePreview() {
	if m.screen != screenFiles || len(m.files) == 0 {
		return
	}
	w := m.preview.Width
	if w < 1 {
		w = 1
	}
	m.preview.SetContent(ansi.Wrap(m.previewContent(m.fileIdx), w, " -"))
}

// refreshFileList rebuilds the left-list viewport content. Called whenever the
// list contents or cursor highlight changes.
func (m *model) refreshFileList() {
	m.fileList.SetContent(m.fileListContent())
}

// refreshToolList rebuilds the tools-list viewport content. Called whenever
// the tool rows or cursor highlight changes.
func (m *model) refreshToolList() {
	m.toolList.SetContent(strings.Join(m.toolRowLines(), "\n"))
}

// followFileCursor scrolls m.fileList so the row for m.fileIdx stays visible.
func (m *model) followFileCursor() {
	if len(m.files) == 0 || m.fileList.Height <= 0 {
		return
	}
	y := m.fileRowYOffset(m.fileIdx)
	if y < m.fileList.YOffset {
		m.fileList.SetYOffset(y)
	} else if y >= m.fileList.YOffset+m.fileList.Height {
		m.fileList.SetYOffset(y - m.fileList.Height + 1)
	}
}

// followToolCursor scrolls m.toolList so the row for m.toolIdx stays visible.
// Each tool row is exactly one line so y == toolIdx.
func (m *model) followToolCursor() {
	if len(m.toolItems) == 0 || m.toolList.Height <= 0 {
		return
	}
	y := m.toolIdx
	if y < m.toolList.YOffset {
		m.toolList.SetYOffset(y)
	} else if y >= m.toolList.YOffset+m.toolList.Height {
		m.toolList.SetYOffset(y - m.toolList.Height + 1)
	}
}

// setScreen swaps the active screen and refreshes viewport contents so a
// freshly-shown screen reflects the latest data without waiting for a reload.
func (m *model) setScreen(s screen) {
	m.screen = s
	m.refreshFileList()
	m.refreshToolList()
	m.updatePreview()
}

// computeLayout derives the on-screen geometry of every clickable / scrollable
// region from m.w / m.h and the lipgloss styles used by View(). Pure — no
// model mutation. Caller hits results against MouseMsg X/Y.
func (m model) computeLayout() layoutMetrics {
	var l layoutMetrics
	l.tabRowY = 0
	l.bodyTopY = 1
	// Body panel height is m.h - 4 (1 tab row + 1 footer row + 2 spacing rows
	// outside the panel). Body occupies rows [1, m.h-3).
	l.bodyBottomY = l.bodyTopY + (m.h - 4)

	labels := []string{"[1] Files", "[2] Tools", "[3] Sync"}
	x := 0
	l.tabXRanges = make([]tabRange, len(labels))
	for i, lab := range labels {
		var w int
		if screen(i) == m.screen {
			w = lipgloss.Width(styleTabActive.Render(lab))
		} else {
			w = lipgloss.Width(styleTab.Render(lab))
		}
		l.tabXRanges[i] = tabRange{start: x, end: x + w}
		x += w
	}
	scopeStr := fmt.Sprintf("  scope: %s  [g]", m.scope)
	sw := lipgloss.Width(lipgloss.NewStyle().Foreground(colorMuted).Render(scopeStr))
	l.scopeXRange = tabRange{start: x, end: x + sw}

	// Files: outer panel sits at MarginLeft(1). Each panel has 1-cell border
	// on each side, so the inner content starts at panelX + 1, panelTopY + 1.
	leftW := m.w/3 - 2
	rightW := m.w - leftW - 7
	l.filesPanelTopY = l.bodyTopY
	l.filesPanelBottomY = l.bodyBottomY
	l.filesLeftPanelX = 1
	l.filesLeftPanelW = leftW
	l.filesRightPanelX = l.filesLeftPanelX + leftW
	l.filesRightPanelW = rightW
	l.filesListInnerY0 = l.filesPanelTopY + 1

	// Tools: also MarginLeft(1).
	l.toolsPanelX = 1
	l.toolsPanelW = m.w - 5

	// Sync: stylePanelBorderInset bakes in MarginLeft(1).
	l.syncPanelX = 1
	l.syncPanelW = m.w - 5
	return l
}

// handleMouse routes a MouseMsg to the click or wheel handler appropriate to
// the screen region under (X, Y).
func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	l := m.computeLayout()

	// Tab row + scope label sit above the screen body and apply on every screen.
	if msg.Y == l.tabRowY {
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		for i, r := range l.tabXRanges {
			if msg.X >= r.start && msg.X < r.end {
				m.setScreen(screen(i))
				return m, nil
			}
		}
		if msg.X >= l.scopeXRange.start && msg.X < l.scopeXRange.end {
			return m.toggleScope()
		}
		return m, nil
	}

	switch m.screen {
	case screenFiles:
		return m.handleFilesMouse(msg, l)
	case screenTools:
		return m.handleToolsMouse(msg, l)
	case screenSync:
		return m.handleSyncMouse(msg, l)
	}
	return m, nil
}

func (m model) handleFilesMouse(msg tea.MouseMsg, l layoutMetrics) (tea.Model, tea.Cmd) {
	inLeft := msg.X >= l.filesLeftPanelX && msg.X < l.filesLeftPanelX+l.filesLeftPanelW &&
		msg.Y >= l.filesPanelTopY && msg.Y < l.filesPanelBottomY
	inRight := msg.X >= l.filesRightPanelX && msg.X < l.filesRightPanelX+l.filesRightPanelW &&
		msg.Y >= l.filesPanelTopY && msg.Y < l.filesPanelBottomY

	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		if inLeft {
			innerY := msg.Y - l.filesListInnerY0 + m.fileList.YOffset
			if i := m.fileIdxAtListY(innerY); i >= 0 {
				m.fileIdx = i
				m.followFileCursor()
				m.refreshFileList()
				m.updatePreview()
			}
		}
		return m, nil
	case tea.MouseButtonWheelUp:
		if inLeft {
			m.fileList.ScrollUp(3)
		} else if inRight {
			m.preview.ScrollUp(3)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if inLeft {
			m.fileList.ScrollDown(3)
		} else if inRight {
			m.preview.ScrollDown(3)
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleToolsMouse(msg tea.MouseMsg, l layoutMetrics) (tea.Model, tea.Cmd) {
	inPanel := msg.X >= l.toolsPanelX && msg.X < l.toolsPanelX+l.toolsPanelW &&
		msg.Y >= l.bodyTopY && msg.Y < l.bodyBottomY
	if !inPanel {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.toolList.ScrollUp(3)
	case tea.MouseButtonWheelDown:
		m.toolList.ScrollDown(3)
	}
	return m, nil
}

func (m model) handleSyncMouse(msg tea.MouseMsg, l layoutMetrics) (tea.Model, tea.Cmd) {
	inPanel := msg.X >= l.syncPanelX && msg.X < l.syncPanelX+l.syncPanelW &&
		msg.Y >= l.bodyTopY && msg.Y < l.bodyBottomY
	if !inPanel {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.logView.ScrollUp(3)
	case tea.MouseButtonWheelDown:
		m.logView.ScrollDown(3)
	}
	return m, nil
}

// resizeViewports updates the dimensions of every viewport to match m.w / m.h.
// Sizes mirror what viewFiles / viewTools / viewSync render so wheel + Ctrl+D/U
// page over the same visible window the user sees.
func (m *model) resizeViewports() {
	panelOuterH := m.h - 4
	panelInnerH := panelOuterH - 2 // minus top + bottom border

	// Files left list.
	leftW := m.w/3 - 2
	leftInnerW := leftW - 2
	if leftInnerW < 1 {
		leftInnerW = 1
	}
	listH := panelInnerH
	if listH < 1 {
		listH = 1
	}
	m.fileList.Width = leftInnerW
	m.fileList.Height = listH

	// Files preview (right pane).
	rightW := m.w - leftW - 7
	const previewPadX = 2
	previewInnerW := rightW - 2*previewPadX
	if previewInnerW < 1 {
		previewInnerW = 1
	}
	m.preview.Width = previewInnerW
	// Height is recomputed in viewFiles based on badge count; set a sensible
	// default here so HalfPageDown works even before the first View() call.
	m.preview.Height = panelInnerH

	// Sync log.
	m.logView.Width = m.w - 7
	m.logView.Height = m.h - 9

	// Tools list. Outer panel width is m.w - 5; inside the border + the title
	// row + a blank separator, the rows themselves get panelInnerH-2.
	toolsInnerW := (m.w - 5) - 2
	if toolsInnerW < 1 {
		toolsInnerW = 1
	}
	toolsInnerH := panelInnerH - 2
	if toolsInnerH < 1 {
		toolsInnerH = 1
	}
	m.toolList.Width = toolsInnerW
	m.toolList.Height = toolsInnerH

	// Editor (Files right pane when m.editing).
	m.editor.SetWidth(previewInnerW)
	m.editor.SetHeight(panelInnerH)
}

func (m model) startEdit() model {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return m
	}
	if m.files[m.fileIdx].placeholder {
		return m
	}
	content := m.fileContent(m.fileIdx)
	m.editBody = content
	m.editor.SetValue(content)
	m.editor.Focus()
	m.editing = true
	return m
}

// startNewFile opens the textinput modal for creating a new file in the
// group at the current cursor position. AGENTS.md is a single-file concept
// so it's a no-op there.
func (m model) startNewFile() model {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return m
	}
	kind := m.files[m.fileIdx].kind
	if kind == kindAgentsMD {
		return m
	}
	m.inputKind = kind
	m.inputErr = ""
	m.input.Reset()
	m.input.Placeholder = newFilePlaceholder(kind)
	// fit textinput to modal interior (width minus border + padding)
	if w := modalWidth(m.w) - 6; w > 10 {
		m.input.Width = w
	}
	m.input.Focus()
	m.inputting = true
	return m
}

// newFilePlaceholder returns the textinput placeholder text for a kind.
func newFilePlaceholder(k fileKind) string {
	switch k {
	case kindSkill:
		return "skill folder name (e.g. release-prep)"
	case kindAgent:
		return "subagent slug (e.g. adapter-reviewer)"
	case kindCommand:
		return "command slug (e.g. ship)"
	case kindRule:
		return "rule slug (e.g. style-guide)"
	}
	return ""
}

// updateInput handles key dispatch while the new-file textinput modal is open.
func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.inputting = false
			m.inputErr = ""
			m.input.Reset()
			m.input.Blur()
			return m, nil
		case "enter":
			return m.submitNewFile()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// submitNewFile validates the typed name, creates the canonical file on disk,
// and triggers a reload that will position the cursor and open the editor.
func (m model) submitNewFile() (tea.Model, tea.Cmd) {
	slug, err := validateNewName(m.inputKind, m.input.Value(), m.canonical)
	if err != nil {
		m.inputErr = err.Error()
		return m, nil
	}
	switch m.inputKind {
	case kindSkill:
		if _, err := canonical.CreateEmptySkill(m.workspace, slug); err != nil {
			m.inputErr = err.Error()
			return m, nil
		}
	case kindAgent:
		if _, err := canonical.CreateEmptyAgent(m.workspace, slug); err != nil {
			m.inputErr = err.Error()
			return m, nil
		}
	case kindCommand:
		if _, err := canonical.CreateEmptyCommand(m.workspace, slug); err != nil {
			m.inputErr = err.Error()
			return m, nil
		}
	case kindRule:
		if _, err := canonical.CreateEmptyRule(m.workspace, slug); err != nil {
			m.inputErr = err.Error()
			return m, nil
		}
	default:
		m.inputErr = "cannot create files of this kind"
		return m, nil
	}
	m.inputting = false
	m.input.Reset()
	m.input.Blur()
	m.pendingSelect = &pendingSel{kind: m.inputKind, slug: slug}
	return m, reloadCmd(m.workspace)
}

// validateNewName cleans and validates a user-typed slug for a new file.
// Rules: trim, strip trailing .md, must match [a-z0-9._-]+, reject "."/"..",
// reject reserved rule names, reject duplicates against existing canonical.
func validateNewName(kind fileKind, raw string, c *canonical.Canonical) (string, error) {
	slug := strings.TrimSpace(raw)
	if slug == "" {
		return "", fmt.Errorf("name required")
	}
	slug = strings.TrimSuffix(slug, ".md")
	if slug == "" {
		return "", fmt.Errorf("name required")
	}
	if slug == "." || slug == ".." {
		return "", fmt.Errorf("invalid name")
	}
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return "", fmt.Errorf("use lowercase letters, digits, dot, underscore, hyphen")
		}
	}
	if kind == kindRule && canonical.IsReservedRuleName(slug) {
		return "", fmt.Errorf("'%s' is reserved (Cursor catch-all)", slug)
	}
	if c != nil {
		switch kind {
		case kindSkill:
			for _, s := range c.Skills {
				if s.Dir == slug {
					return "", fmt.Errorf("skill '%s' already exists", slug)
				}
			}
		case kindAgent:
			for _, a := range c.Agents {
				if a.Filename == slug {
					return "", fmt.Errorf("subagent '%s' already exists", slug)
				}
			}
		case kindCommand:
			for _, cmd := range c.Commands {
				if cmd.Filename == slug {
					return "", fmt.Errorf("command '%s' already exists", slug)
				}
			}
		case kindRule:
			for _, r := range c.Rules {
				if r.Filename == slug {
					return "", fmt.Errorf("rule '%s' already exists", slug)
				}
			}
		}
	}
	return slug, nil
}

// findFileIndex returns the position of the fileItem matching kind+slug
// (Skill.Dir, or Filename for the others), or -1 if not found.
func findFileIndex(files []fileItem, kind fileKind, slug string) int {
	for i, f := range files {
		if f.kind != kind || f.placeholder {
			continue
		}
		switch kind {
		case kindSkill:
			if f.skill != nil && f.skill.Dir == slug {
				return i
			}
		case kindAgent:
			if f.agent != nil && f.agent.Filename == slug {
				return i
			}
		case kindCommand:
			if f.command != nil && f.command.Filename == slug {
				return i
			}
		case kindRule:
			if f.rule != nil && f.rule.Filename == slug {
				return i
			}
		}
	}
	return -1
}

// startDeleteFile opens the confirm modal for the file at the cursor.
// AGENTS.md and placeholder rows are not deletable.
func (m model) startDeleteFile() model {
	if m.fileIdx < 0 || m.fileIdx >= len(m.files) {
		return m
	}
	f := m.files[m.fileIdx]
	if f.placeholder || f.kind == kindAgentsMD {
		return m
	}
	m.confirmingDelete = true
	m.deleteTarget = m.fileIdx
	m.deleteErr = ""
	return m
}

// updateDeleteConfirm handles key dispatch while the delete confirm modal is open.
func (m model) updateDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "y", "Y", "enter":
		return m.confirmDelete()
	case "n", "N", "esc", "q":
		m.confirmingDelete = false
		m.deleteErr = ""
		return m, nil
	}
	return m, nil
}

// confirmDelete removes the canonical file/folder for the targeted item and
// triggers a reload. The cursor is moved up by one when possible so it lands
// on a sensible neighbor after the list rebuilds.
func (m model) confirmDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget < 0 || m.deleteTarget >= len(m.files) {
		m.confirmingDelete = false
		return m, nil
	}
	f := m.files[m.deleteTarget]
	var err error
	switch f.kind {
	case kindSkill:
		if f.skill != nil {
			err = canonical.DeleteSkill(m.workspace, f.skill.Dir)
		}
	case kindAgent:
		if f.agent != nil {
			err = canonical.DeleteAgent(m.workspace, f.agent.Filename)
		}
	case kindCommand:
		if f.command != nil {
			err = canonical.DeleteCommand(m.workspace, f.command.Filename)
		}
	case kindRule:
		if f.rule != nil {
			err = canonical.DeleteRule(m.workspace, f.rule.Filename)
		}
	default:
		m.confirmingDelete = false
		return m, nil
	}
	if err != nil {
		m.deleteErr = err.Error()
		return m, nil
	}
	m.confirmingDelete = false
	if m.fileIdx > 0 {
		m.fileIdx--
	}
	return m, reloadCmd(m.workspace)
}

// deleteTargetLabel returns the label shown in the confirm modal.
func (m model) deleteTargetLabel() string {
	if m.deleteTarget < 0 || m.deleteTarget >= len(m.files) {
		return ""
	}
	f := m.files[m.deleteTarget]
	if f.kind == kindSkill && f.skill != nil {
		return "skills/" + f.skill.Dir + "/  (entire folder)"
	}
	return f.label
}

func (m model) toggleTool() model {
	if m.toolIdx >= len(m.toolItems) {
		return m
	}
	item := &m.toolItems[m.toolIdx]
	item.enabled = !item.enabled
	tc := m.config.Tools[item.adapter.Meta.Name]
	tc.Enabled = item.enabled
	m.config.Tools[item.adapter.Meta.Name] = tc
	_ = config.Save(m.workspace, m.config)
	m.refreshToolList()
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
	m.inputting = false
	m.inputErr = ""
	m.input.Reset()
	m.input.Blur()
	m.pendingSelect = nil
	m.confirmingDelete = false
	m.deleteErr = ""
	m.inactive = &leaving
	m.fileList.SetYOffset(0)
	m.toolList.SetYOffset(0)
	m.refreshFileList()
	m.refreshToolList()
	m.updatePreview()

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

	if m.scope == tools.ScopeProject && !m.config.Gitignore.Prompted {
		m.showGitignorePrompt = true
		return m, nil
	}

	if m.scope == tools.ScopeProject && m.config.Gitignore.Manage {
		if err := gitignore.Update(m.workspace, gitignore.Compute(m.adapters)); err != nil {
			m.syncLines = append(m.syncLines, fmt.Sprintf("  ✗ gitignore: %v", err))
		}
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
func buildSyncLines(result *syncer.SyncResult, adapters []tools.Tool) []string {
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
		name := a.Meta.Name
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
	case "Mistral Vibe":
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
	if m.inputting {
		view = m.overlayInputModal(view)
	}
	if m.confirmingDelete {
		view = m.overlayDeleteConfirm(view)
	}
	if m.showDiv {
		view = m.overlayDivModal(view)
	}
	if m.showToolInfo {
		view = m.overlayToolInfo(view)
	}
	if m.showGitignorePrompt {
		view = m.overlayGitignorePrompt(view)
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

type keyHint struct{ key, label string }

const (
	footerHintGap  = "   "
	footerGroupGap = "       "
)

func renderKeyHint(h keyHint) string {
	return styleFooterKey.Render(h.key) + " " + styleFooterLabel.Render(h.label)
}

func joinKeyGroups(groups ...[]keyHint) string {
	rendered := make([]string, 0, len(groups))
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		hints := make([]string, len(g))
		for i, h := range g {
			hints[i] = renderKeyHint(h)
		}
		rendered = append(rendered, strings.Join(hints, footerHintGap))
	}
	return strings.Join(rendered, footerGroupGap)
}

func (m model) renderFooter() string {
	var keys string
	switch {
	case m.editing:
		keys = joinKeyGroups(
			[]keyHint{{"ctrl+s", "save"}},
			[]keyHint{{"esc", "cancel"}},
		)
	case m.inputting:
		keys = joinKeyGroups(
			[]keyHint{{"enter", "create"}},
			[]keyHint{{"esc", "cancel"}},
		)
	case m.confirmingDelete:
		keys = joinKeyGroups(
			[]keyHint{{"y", "confirm"}},
			[]keyHint{{"n/esc", "cancel"}},
		)
	case m.showDiv:
		keys = joinKeyGroups(
			[]keyHint{{"a", "adopt"}, {"o", "overwrite"}, {"d", "defer"}},
			[]keyHint{{"enter", "apply"}},
			[]keyHint{{"esc", "cancel"}},
		)
	case m.showGitignorePrompt:
		keys = joinKeyGroups(
			[]keyHint{{"a", "apply"}, {"s", "skip"}},
			[]keyHint{{"esc", "dismiss"}},
		)
	case m.screen == screenFiles:
		canCreate := len(m.files) > 0 && m.files[m.fileIdx].kind != kindAgentsMD
		canDelete := canCreate && !m.files[m.fileIdx].placeholder
		canCopy := len(m.files) > 0 && !m.files[m.fileIdx].placeholder
		actions := []keyHint{}
		if canCreate {
			actions = append(actions, keyHint{"n", "new"})
		}
		actions = append(actions, keyHint{"e", "edit"})
		if canDelete {
			actions = append(actions, keyHint{"d", "delete"})
		}
		if canCopy {
			actions = append(actions, keyHint{"c", "copy"})
		}
		keys = joinKeyGroups(
			actions,
			[]keyHint{{"s", "sync"}},
			[]keyHint{{"ctrl+u/d", "scroll"}},
			[]keyHint{{"q", "quit"}},
		)
	case m.showToolInfo:
		keys = joinKeyGroups(
			[]keyHint{{"esc/enter", "close"}},
		)
	case m.screen == screenTools:
		keys = joinKeyGroups(
			[]keyHint{{"space", "toggle"}, {"enter", "info"}},
			[]keyHint{{"s", "sync"}},
			[]keyHint{{"ctrl+u/d", "scroll"}},
			[]keyHint{{"q", "quit"}},
		)
	case m.screen == screenSync:
		keys = joinKeyGroups(
			[]keyHint{{"s", "sync"}},
			[]keyHint{{"ctrl+u/d", "scroll"}},
			[]keyHint{{"q", "quit"}},
		)
	}
	rendered := styleFooter.Render(keys)
	if m.flash != "" {
		flashColor := colorSuccess
		if strings.HasPrefix(m.flash, "✗") {
			flashColor = colorDanger
		}
		rendered += "  " + lipgloss.NewStyle().Foreground(flashColor).Render(m.flash)
	}
	return rendered
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

// fileListContent renders the left-list block (group headers + file rows)
// the same way viewFiles does. Pure helper so refresh hooks can stamp the
// content into m.fileList without going through View().
func (m model) fileListContent() string {
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
		label := f.label
		if f.placeholder {
			label = stylePlaceholderRow.Render(f.label)
		}
		var row string
		if i == m.fileIdx {
			selected := f.label
			if f.placeholder {
				selected = stylePlaceholderRow.Render(f.label)
			} else {
				selected = styleSelected.Render(f.label)
			}
			row = styleCursorMark.Render("▍ ") + icon + "  " + selected
		} else {
			row = "  " + icon + "  " + label
		}
		listLines = append(listLines, row)
	}
	return strings.Join(listLines, "\n")
}

// fileRowYOffset returns the 0-based Y inside fileListContent() where the row
// for m.files[idx] is drawn. Each file row is 1 line; a group transition adds
// 1 (header) or 2 (blank + header) lines before the row.
func (m model) fileRowYOffset(idx int) int {
	if idx < 0 || idx >= len(m.files) {
		return 0
	}
	y := 0
	prev := fileKind(-1)
	for i, f := range m.files {
		if hdr := groupHeader(prev, f.kind); hdr != "" {
			if prev != fileKind(-1) {
				y++ // blank separator
			}
			y++ // header line
		}
		if i == idx {
			return y
		}
		y++
		prev = f.kind
	}
	return y
}

// fileIdxAtListY reverse-maps a Y inside fileListContent() to a file index, or
// -1 if Y falls on a header / blank-separator line.
func (m model) fileIdxAtListY(y int) int {
	if y < 0 || len(m.files) == 0 {
		return -1
	}
	cur := 0
	prev := fileKind(-1)
	for i, f := range m.files {
		if hdr := groupHeader(prev, f.kind); hdr != "" {
			if prev != fileKind(-1) {
				if cur == y {
					return -1
				}
				cur++
			}
			if cur == y {
				return -1
			}
			cur++
		}
		if cur == y {
			return i
		}
		cur++
		prev = f.kind
	}
	return -1
}

// computeBadges returns the per-tool compatibility badges for the currently
// selected file. Empty when no files exist.
func (m model) computeBadges() []string {
	if len(m.files) == 0 {
		return nil
	}
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
	var badges []string
	for _, a := range m.adapters {
		compat := a.Meta.Supports(concept)
		switch {
		case compat.Supported && !compat.Deprecated:
			label := a.Meta.Name
			if f.kind == kindRule {
				if notice := ruleAppendNotice(a.Meta.Name); notice != "" {
					label = fmt.Sprintf("%s (%s)", a.Meta.Name, notice)
				}
			} else {
				if alias := a.Meta.Alias(concept); alias != "" {
					label = fmt.Sprintf("%s (%s)", a.Meta.Name, alias)
				}
			}
			badges = append(badges, fmt.Sprintf("%s %s", styleBadgeOk, label))
		case compat.Deprecated:
			badges = append(badges, fmt.Sprintf("%s %s — %s", styleBadgeWarn, a.Meta.Name, compat.Reason))
		default:
			badges = append(badges, fmt.Sprintf("%s %s — %s", styleBadgeFail, a.Meta.Name, compat.Reason))
		}
	}
	return badges
}

func (m model) viewFiles() string {
	leftW := m.w/3 - 2
	rightW := m.w - leftW - 7

	leftPanel := stylePanelBorderActive.Width(leftW).Height(m.h - 4).Render(m.fileList.View())

	const previewPadX = 2
	previewInnerW := rightW - 2*previewPadX
	if previewInnerW < 1 {
		previewInnerW = 1
	}
	rightPanelStyle := stylePanelBorder.Padding(0, previewPadX)
	var rightPanel string
	if m.editing {
		m.editor.SetWidth(previewInnerW)
		rightPanel = rightPanelStyle.Width(rightW).Height(m.h - 4).Render(m.editor.View())
	} else {
		badges := m.computeBadges()
		// Recompute preview Height to account for the badge block + the two blank
		// rows between badges and the preview body.
		m.preview.Height = m.h - 4 - len(badges) - 3
		if m.preview.Height < 1 {
			m.preview.Height = 1
		}
		badgeStr := strings.Join(badges, "\n")
		rightContent := badgeStr + "\n\n" + m.preview.View()
		rightPanel = rightPanelStyle.Width(rightW).Height(m.h - 4).Render(rightContent)
	}

	return lipgloss.NewStyle().MarginLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel))
}

// toolRowLines builds the per-tool row strings (no title, no leading blank).
// Returned slice is what the toolList viewport renders.
func (m model) toolRowLines() []string {
	rows := make([]string, 0, len(m.toolItems))
	for i, item := range m.toolItems {
		check := "[ ]"
		if item.enabled {
			check = lipgloss.NewStyle().Foreground(colorSuccess).Render("[x]")
		}

		installed := lipgloss.NewStyle().Foreground(colorMuted).Render("not installed")
		if item.install.Found {
			installed = lipgloss.NewStyle().Foreground(colorSuccess).Render("installed")
		}

		concepts := []tools.Concept{tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands}
		conceptLabels := []string{"rules", "skills", "agents", "commands"}
		var conceptStr []string
		for ci, concept := range concepts {
			compat := item.adapter.Meta.Supports(concept)
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
			name := styleSelected.Render(fmt.Sprintf("%-14s", item.adapter.Meta.Name))
			row = styleCursorMark.Render("▍ ") + check + "  " + name + "  " + installed + "  " + strings.Join(conceptStr, "  ")
		} else {
			row = "  " + fmt.Sprintf("%s  %-14s  %-15s  %s", check, item.adapter.Meta.Name, installed, strings.Join(conceptStr, "  "))
		}
		rows = append(rows, row)
	}
	return rows
}

func (m model) viewTools() string {
	header := styleTitle.Render("Agent Sync Targets") + "\n\n"
	inner := header + m.toolList.View()
	return stylePanelBorder.Width(m.w - 5).Height(m.h - 4).MarginLeft(1).Render(inner)
}

func (m model) viewSync() string {
	header := styleTitle.Render("Sync")
	bannerLines := 0
	if banner := m.renderGitignoreBanner(); banner != "" {
		header += "\n\n" + banner
		bannerLines = 2
	}
	if !m.syncDone && len(m.syncLines) == 0 {
		header += "\n\nPress [s] to sync canonical → all enabled tool folders."
	}
	m.logView.Width = m.w - 7
	m.logView.Height = m.h - 9 - bannerLines
	m.logView.SetContent(strings.Join(m.syncLines, "\n"))
	content := header + "\n\n" + m.logView.View()
	return stylePanelBorderInset.Width(m.w - 5).Height(m.h - 4).Render(content)
}

// renderGitignoreBanner returns the muted one-line gitignore status banner
// shown above the sync log. Returns "" at user scope (gitignore is never
// touched there).
func (m model) renderGitignoreBanner() string {
	if m.scope != tools.ScopeProject {
		return ""
	}
	mute := lipgloss.NewStyle().Foreground(colorMuted)
	if !m.config.Gitignore.Prompted {
		return mute.Render(".gitignore management: not configured — choose on first sync")
	}
	if !m.config.Gitignore.Manage {
		return mute.Render(".gitignore management: off — edit config.yaml to re-enable")
	}
	entries := gitignore.Compute(m.adapters)
	preview := entries
	if len(preview) > 4 {
		preview = preview[:4]
	}
	suffix := ""
	if len(entries) > len(preview) {
		suffix = fmt.Sprintf(", +%d more", len(entries)-len(preview))
	}
	return mute.Render(fmt.Sprintf("agentsync manages .gitignore for %d entries (%s%s)", len(entries), strings.Join(preview, ", "), suffix))
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
	return placeOverlay(base, modal, m.w, m.h)
}

// placeOverlay composites overlay onto base, centered in a w×h frame, so the
// background TUI remains visible around (and behind transparent edges of) the
// modal. Both inputs may contain ANSI escape codes — line splicing uses
// ansi.Cut so SGR state in either input doesn't leak across the splice.
func placeOverlay(base, overlay string, w, h int) string {
	overlayLines := strings.Split(overlay, "\n")
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := len(overlayLines)

	x := (w - overlayWidth) / 2
	y := (h - overlayHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	baseLines := strings.Split(base, "\n")
	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}

	const sgrReset = "\x1b[0m"
	for i, ovLine := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		bg := baseLines[row]
		bgWidth := lipgloss.Width(bg)
		ovWidth := lipgloss.Width(ovLine)

		// Pad bg right edge with spaces so the splice has somewhere to land.
		needed := x + ovWidth
		if bgWidth < needed {
			bg += strings.Repeat(" ", needed-bgWidth)
			bgWidth = needed
		}

		left := ansi.Cut(bg, 0, x)
		right := ""
		if bgWidth > x+ovWidth {
			right = ansi.Cut(bg, x+ovWidth, bgWidth)
		}
		baseLines[row] = left + sgrReset + ovLine + sgrReset + right
	}

	return strings.Join(baseLines, "\n")
}

// overlayToolInfo renders a centered modal describing the currently-selected
// tool's per-concept output paths and any deviations from the obvious default.
func (m model) overlayToolInfo(base string) string {
	if m.toolIdx < 0 || m.toolIdx >= len(m.toolItems) {
		return base
	}
	item := m.toolItems[m.toolIdx]
	a := item.adapter

	width := m.w * 2 / 3
	if width < 50 {
		width = 50
	}
	if width > m.w-4 {
		width = m.w - 4
	}

	var lines []string
	lines = append(lines, styleInputLabel.Render(a.Meta.Name))

	scopeCompat := a.Meta.SupportsScope(m.scope)
	installStatus := "not installed"
	if item.install.Found {
		installStatus = "installed"
	}
	statusLine := fmt.Sprintf("%s · %s scope", installStatus, m.scope)
	lines = append(lines, lipgloss.NewStyle().Foreground(colorMuted).Render(statusLine))
	if !scopeCompat.Supported && scopeCompat.Reason != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorWarn).Render("⚠ "+scopeCompat.Reason))
	}
	lines = append(lines, "")

	concepts := []tools.Concept{tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands}
	conceptLabels := map[tools.Concept]string{
		tools.ConceptRules:    "rules",
		tools.ConceptSkills:   "skills",
		tools.ConceptAgents:   "agents",
		tools.ConceptCommands: "commands",
	}
	// Modal Width(width) = content + padding (border is added on top by lipgloss).
	// Inner text area = width - padding(4). Each detail line is prefixed with a
	// 4-space indent, so wrap to (innerArea - 4) so prefix+text fits in one line
	// and lipgloss never re-wraps onto an unindented continuation row.
	const indent = "    "
	innerArea := width - 4
	wrapWidth := innerArea - len(indent)
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	for i, concept := range concepts {
		compat := a.Meta.Supports(concept)
		var badge string
		switch {
		case compat.Supported && !compat.Deprecated:
			badge = styleBadgeOk
		case compat.Deprecated:
			badge = styleBadgeWarn
		default:
			badge = styleBadgeFail
		}

		header := badge + " " + lipgloss.NewStyle().Bold(true).Render(conceptLabels[concept])
		if compat.Deprecated && compat.Replacement != "" {
			header += lipgloss.NewStyle().Foreground(colorMuted).Render("  → " + compat.Replacement)
		}
		lines = append(lines, header)

		var detail string
		if info := a.Meta.Info(concept); info != "" {
			detail = info
		} else if compat.Reason != "" {
			detail = compat.Reason
		}
		if detail != "" {
			wrapped := ansi.Wrap(detail, wrapWidth, " -")
			for _, l := range strings.Split(wrapped, "\n") {
				lines = append(lines, indent+lipgloss.NewStyle().Foreground(colorMuted).Render(l))
			}
		}
		if i < len(concepts)-1 {
			lines = append(lines, "")
		}
	}

	modal := styleModalBorder.Width(width).Render(strings.Join(lines, "\n"))
	return placeOverlay(base, modal, m.w, m.h)
}

// overlayGitignorePrompt renders the first-sync modal asking whether agentsync
// should manage a .gitignore block for derived tool dirs/files.
func (m model) overlayGitignorePrompt(base string) string {
	width := m.w * 2 / 3
	if width < 56 {
		width = 56
	}
	if width > m.w-4 {
		width = m.w - 4
	}

	entries := gitignore.Compute(m.adapters)
	mute := lipgloss.NewStyle().Foreground(colorMuted)

	var lines []string
	lines = append(lines, styleInputLabel.Render("Manage .gitignore?"))
	lines = append(lines, mute.Render(fmt.Sprintf("agentsync writes %d derived files/dirs at the workspace root.", len(entries))))
	lines = append(lines, "")
	lines = append(lines, "Apply  — write/refresh a managed block (creates .gitignore if missing).")
	lines = append(lines, "Skip   — remove any existing managed block and don't manage going forward.")
	lines = append(lines, "")
	lines = append(lines, mute.Render("Entries:"))
	for _, e := range entries {
		lines = append(lines, "  "+e)
	}
	lines = append(lines, "", styleFooter.Render("a apply  •  s skip  •  esc dismiss"))

	modal := styleModalBorder.Width(width).Render(strings.Join(lines, "\n"))
	return placeOverlay(base, modal, m.w, m.h)
}

// overlayInputModal renders the new-file textinput modal centered on top of
// the base view.
func (m model) overlayInputModal(base string) string {
	title := newFileTitle(m.inputKind)
	var lines []string
	lines = append(lines,
		styleInputLabel.Render(title),
		"",
		m.input.View(),
	)
	if m.inputErr != "" {
		lines = append(lines, "", styleInputError.Render(m.inputErr))
	}
	lines = append(lines, "", styleFooter.Render("enter create • esc cancel"))

	modal := styleModalBorder.Width(modalWidth(m.w)).Render(strings.Join(lines, "\n"))
	return placeOverlay(base, modal, m.w, m.h)
}

// overlayDeleteConfirm renders the delete confirmation modal centered on the base view.
func (m model) overlayDeleteConfirm(base string) string {
	var lines []string
	lines = append(lines,
		styleInputLabel.Render("Delete file?"),
		"",
		m.deleteTargetLabel(),
	)
	if m.deleteErr != "" {
		lines = append(lines, "", styleInputError.Render(m.deleteErr))
	}
	lines = append(lines, "", styleFooter.Render("y / enter confirm  •  n / esc cancel"))

	modal := styleModalBorder.Width(modalWidth(m.w)).Render(strings.Join(lines, "\n"))
	return placeOverlay(base, modal, m.w, m.h)
}

// modalWidth picks a narrow modal width clamped to a sensible range so the
// box stays compact even on wide terminals.
func modalWidth(termW int) int {
	w := termW * 2 / 5
	if w < 44 {
		w = 44
	}
	if w > 72 {
		w = 72
	}
	if w > termW-4 {
		w = termW - 4
	}
	return w
}

// newFileTitle returns the textinput modal title for a kind.
func newFileTitle(k fileKind) string {
	switch k {
	case kindSkill:
		return "Add new Skill"
	case kindAgent:
		return "Add new Subagent"
	case kindCommand:
		return "Add new Command"
	case kindRule:
		return "Add new Rule"
	}
	return "Add new file"
}

// ---- Run ----

// Run starts the agentsync TUI rooted at workspace with the given initial scope.
// The other scope is lazy-loaded on first toggle. If the active scope's
// .agentsync/ doesn't exist, the TUI launches with an empty-state banner
// instead of failing.
func Run(workspace string, scope tools.Scope, adapters []tools.Tool) error {
	snap := loadScopeSnapshot(workspace, scope)
	if snap.loadErr != nil {
		return fmt.Errorf("load %s scope: %w", scope, snap.loadErr)
	}

	m := initialModel(workspace, scope, snap.canonical, snap.config, adapters)
	m.initialized = snap.initialized
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
