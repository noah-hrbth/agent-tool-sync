package tools

import (
	"fmt"
	"strings"
)

type fmField struct {
	key   string
	value any // string, []string, or bool
}

// buildMDFrontmatter builds a markdown file with YAML frontmatter.
// Skips fields where value is zero (empty string, false, nil/empty slice).
func buildMDFrontmatter(fields []fmField, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, f := range fields {
		switch v := f.value.(type) {
		case string:
			if v != "" {
				fmt.Fprintf(&sb, "%s: %s\n", f.key, v)
			}
		case []string:
			if len(v) > 0 {
				fmt.Fprintf(&sb, "%s: [%s]\n", f.key, strings.Join(v, ", "))
			}
		case bool:
			if v {
				fmt.Fprintf(&sb, "%s: true\n", f.key)
			}
		}
	}
	sb.WriteString("---\n")
	if body != "" {
		sb.WriteString(body)
	}
	return sb.String()
}

// buildTOML writes a flat TOML key = value document from fields.
// Skips fields where value is zero (empty string, false, nil/empty slice).
func buildTOML(fields []fmField) string {
	var sb strings.Builder
	for _, f := range fields {
		switch v := f.value.(type) {
		case string:
			if v == "" {
				continue
			}
			if strings.Contains(v, "\n") {
				// multiline: prefer literal block; fall back to basic block if unsafe
				if !strings.Contains(v, "'''") && !strings.HasSuffix(v, "'") {
					fmt.Fprintf(&sb, "%s = '''\n%s\n'''\n", f.key, v)
				} else {
					escaped := strings.ReplaceAll(v, `\`, `\\`)
					escaped = strings.ReplaceAll(escaped, `"`, `\"`)
					fmt.Fprintf(&sb, "%s = \"\"\"\n%s\n\"\"\"\n", f.key, escaped)
				}
			} else {
				escaped := strings.ReplaceAll(v, `\`, `\\`)
				escaped = strings.ReplaceAll(escaped, `"`, `\"`)
				fmt.Fprintf(&sb, "%s = \"%s\"\n", f.key, escaped)
			}
		case []string:
			// Not used for TOML output currently; skip.
		case bool:
			if v {
				fmt.Fprintf(&sb, "%s = true\n", f.key)
			}
		}
	}
	return sb.String()
}
