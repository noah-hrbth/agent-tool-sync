package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

// appendSkillDocs appends one FileWrite per skill doc under skillDir (the dir
// holding SKILL.md), preserving each doc's relative path. Skill docs are plain
// markdown — no frontmatter. Shared by every skill-rendering tool so doc output
// stays uniform across adapters.
func appendSkillDocs(files []FileWrite, skillDir string, docs []canonical.SkillDoc) []FileWrite {
	for _, d := range docs {
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(skillDir, filepath.FromSlash(d.RelPath)),
			Content: []byte(d.Content),
		})
	}
	return files
}
