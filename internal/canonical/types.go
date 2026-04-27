package canonical

// Rule is a rule definition from .agentsync/rules/<filename>.md.
type Rule struct {
	Filename    string   `yaml:"-"` // basename without .md (set by loader)
	Description string   `yaml:"description,omitempty"`
	Paths       []string `yaml:"paths,omitempty"` // optional glob targeting
	Body        string   `yaml:"-"`
}

// Skill is a skill definition from .agentsync/skills/<dir>/SKILL.md.
type Skill struct {
	Dir                    string   `yaml:"-"` // folder name under skills/ (set by loader)
	Name                   string   `yaml:"name"`
	Description            string   `yaml:"description"`
	AllowedTools           []string `yaml:"allowed-tools,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	Paths                  []string `yaml:"paths,omitempty"`
	Body                   string   `yaml:"-"` // markdown body after frontmatter
}

// Agent is a subagent definition from .agentsync/agents/<filename>.md.
type Agent struct {
	Filename    string   `yaml:"-"` // basename without extension (set by loader)
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tools       []string `yaml:"tools,omitempty"`
	Model       string   `yaml:"model,omitempty"`
	Body        string   `yaml:"-"`
}

// Command is a slash command definition from .agentsync/commands/<filename>.md.
type Command struct {
	Filename     string   `yaml:"-"` // basename without extension (set by loader)
	Description  string   `yaml:"description"`
	ArgumentHint string   `yaml:"argument-hint,omitempty"`
	AllowedTools []string `yaml:"allowed-tools,omitempty"`
	Model        string   `yaml:"model,omitempty"`
	Body         string   `yaml:"-"`
}

// Canonical holds the full parsed state from .agentsync/.
type Canonical struct {
	Workspace string
	AgentsMD  string // content of .agentsync/AGENTS.md
	Rules     []*Rule
	Skills    []*Skill
	Agents    []*Agent
	Commands  []*Command
}
