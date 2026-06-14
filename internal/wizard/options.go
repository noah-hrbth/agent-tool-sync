package wizard

import "github.com/noah-hrbth/agentsync/internal/tools"

// SourceOption is one selectable import source in the wizard's tool step.
type SourceOption struct {
	Tool        tools.Tool
	Recommended bool // true for claude
}

// DetectedNames returns the display names of every tool detected at the given
// scope, in registry order. Lighter than BuildOptions for callers (e.g. the
// headless --from path) that need detection-based enablement but not the
// import-eligible option list.
func DetectedNames(ws string, scope tools.Scope) []string {
	var names []string
	for _, t := range tools.All() {
		if tools.DetectAtScope(ws, t, scope).Found {
			names = append(names, t.Meta.Name)
		}
	}
	return names
}

// BuildOptions probes every registered tool at the given scope and returns the
// names of all detected tools plus the import-eligible subset as wizard
// options. Options keep registry order except claude, which is pinned first
// and marked Recommended.
func BuildOptions(ws string, scope tools.Scope) (detectedNames []string, options []SourceOption) {
	for _, t := range tools.All() {
		if !tools.DetectAtScope(ws, t, scope).Found {
			continue
		}
		detectedNames = append(detectedNames, t.Meta.Name)
		if !tools.ImportEligible(t, scope) {
			continue
		}
		if t.Meta.Key == "claude" {
			options = append([]SourceOption{{Tool: t, Recommended: true}}, options...)
			continue
		}
		options = append(options, SourceOption{Tool: t})
	}
	return detectedNames, options
}
