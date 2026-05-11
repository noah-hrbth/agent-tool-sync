package gitignore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// blockRegex matches a single agentsync-managed block including its trailing
// newline (if present). Multiple non-overlapping matches are stripped together.
var blockRegex = regexp.MustCompile(`(?s)# BEGIN agentsync managed\n.*?\n# END agentsync managed\n?`)

// multiBlankRegex collapses runs of 3+ consecutive newlines (which appear
// where a block sat between blank lines) down to a single blank line so the
// surrounding user content doesn't end up with widening gaps.
var multiBlankRegex = regexp.MustCompile(`\n{3,}`)

// Update writes or refreshes the agentsync-managed block in
// <workspace>/.gitignore. Creates the file if missing. Idempotent on identical
// input — repeated calls produce byte-identical files.
func Update(workspace string, entries []string) error {
	existing, err := readGitignoreFile(workspace)
	if err != nil {
		return err
	}
	stripped := strings.TrimRight(stripBlocks(existing), "\n")
	block := buildBlock(entries)
	var content string
	if stripped == "" {
		content = block
	} else {
		content = stripped + "\n\n" + block
	}
	return writeAtomic(gitignorePath(workspace), content)
}

// Remove deletes the agentsync-managed block from <workspace>/.gitignore,
// preserving all surrounding lines. No-op when the file is missing or contains
// no managed block (byte-identical preservation).
func Remove(workspace string) error {
	existing, err := readGitignoreFile(workspace)
	if err != nil {
		return err
	}
	if !strings.Contains(existing, BeginMarker) {
		return nil
	}
	return writeAtomic(gitignorePath(workspace), stripBlocks(existing))
}

func gitignorePath(workspace string) string {
	return filepath.Join(workspace, ".gitignore")
}

func readGitignoreFile(workspace string) (string, error) {
	b, err := os.ReadFile(gitignorePath(workspace))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func buildBlock(entries []string) string {
	var b strings.Builder
	b.WriteString(BeginMarker)
	b.WriteByte('\n')
	for _, e := range entries {
		b.WriteString(e)
		b.WriteByte('\n')
	}
	b.WriteString(EndMarker)
	b.WriteByte('\n')
	return b.String()
}

func stripBlocks(content string) string {
	stripped := blockRegex.ReplaceAllString(content, "")
	return multiBlankRegex.ReplaceAllString(stripped, "\n\n")
}

func writeAtomic(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
