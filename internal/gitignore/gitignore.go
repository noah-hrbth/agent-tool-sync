// Package gitignore manages a marker-delimited block inside a workspace's
// .gitignore so derived per-tool directories and files emitted by agentsync
// adapters can be excluded from git without trampling user-managed content.
package gitignore

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// BeginMarker and EndMarker bracket the managed block inside .gitignore.
const (
	BeginMarker = "# BEGIN agentsync managed"
	EndMarker   = "# END agentsync managed"
)

// Compute renders every adapter against a stub canonical at ScopeProject and
// returns a sorted, deduplicated list of gitignore entries:
//   - directories appear as "<seg>/" with a trailing slash
//   - top-level files appear as bare names
//
// `.agentsync` is always excluded (defensive — no adapter should emit it).
// `AGENTS.md` at the workspace root is also excluded per the user-confirmed
// carve-out (it is a shared human-readable spec, commonly committed).
// `.github` is excluded because it is shared with CI workflows that MUST stay
// tracked; the GitHub Copilot adapter writes inside `.github/` for instructions,
// skills, agents, and prompts.
func Compute(adapters []tools.Tool) []string {
	stub := stubCanonical()
	seen := make(map[string]struct{}, 32)
	for _, a := range adapters {
		files, err := a.Render(stub, tools.ScopeProject)
		if err != nil {
			continue
		}
		for _, f := range files {
			if entry := entryFor(f.Path); entry != "" {
				seen[entry] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// entryFor maps a single adapter-emitted relative path to its gitignore entry,
// or "" when the path should be skipped entirely.
func entryFor(path string) string {
	slash := filepath.ToSlash(path)
	if slash == "" {
		return ""
	}
	seg, _, hasSlash := strings.Cut(slash, "/")
	if seg == "" || seg == ".agentsync" || seg == ".github" {
		return ""
	}
	if !hasSlash && seg == "AGENTS.md" {
		return ""
	}
	if hasSlash {
		return seg + "/"
	}
	return seg
}

// stubCanonical returns a canonical populated with one of each entity so every
// adapter renders at least one file per concept. Without this most adapters
// short-circuit on empty input (Claude, OpenCode, etc.) and Compute would
// under-report.
func stubCanonical() *canonical.Canonical {
	return &canonical.Canonical{
		AgentsMD: "stub",
		Rules:    []*canonical.Rule{{Filename: "stub", Description: "stub", Body: "stub"}},
		Skills:   []*canonical.Skill{{Dir: "stub", Name: "stub", Description: "stub", Body: "stub"}},
		Agents:   []*canonical.Agent{{Filename: "stub", Name: "stub", Description: "stub", Body: "stub"}},
		Commands: []*canonical.Command{{Filename: "stub", Description: "stub", Body: "stub"}},
	}
}
