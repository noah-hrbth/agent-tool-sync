package tools

import (
	"fmt"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

type fmField struct {
	key   string
	value any // string, []string, or bool
}

// yamlScalar renders s as a YAML-safe scalar, quoting only when a plain scalar
// would misparse — leading flow indicators ([ {), reserved words (true/yes/null),
// numeric-looking values, embedded ": ", etc. Plain values pass through unquoted.
// Without this, e.g. an argument-hint of "[arg]" emits as a YAML sequence and
// breaks the adopt round-trip (cannot unmarshal !!seq into string).
func yamlScalar(s string) string {
	b, err := yaml.Marshal(s)
	if err != nil {
		return s // unreachable for plain strings
	}
	return strings.TrimRight(string(b), "\n")
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
				fmt.Fprintf(&sb, "%s: %s\n", f.key, yamlScalar(v))
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
					fmt.Fprintf(&sb, "%s = \"\"\"\n%s\n\"\"\"\n", f.key, tomlEscape(v))
				}
			} else {
				fmt.Fprintf(&sb, "%s = \"%s\"\n", f.key, tomlEscape(v))
			}
		case []string:
			if len(v) == 0 {
				continue
			}
			quoted := make([]string, len(v))
			for i, s := range v {
				quoted[i] = `"` + tomlEscape(s) + `"`
			}
			fmt.Fprintf(&sb, "%s = [%s]\n", f.key, strings.Join(quoted, ", "))
		case bool:
			if v {
				fmt.Fprintf(&sb, "%s = true\n", f.key)
			}
		}
	}
	return sb.String()
}

func tomlEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
