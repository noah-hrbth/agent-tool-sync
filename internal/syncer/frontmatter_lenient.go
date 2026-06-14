package syncer

import (
	"strings"

	"github.com/adrg/frontmatter"
)

// globFrontmatterKeys are the frontmatter keys whose values are file globs.
// Hand-authored rule/skill files commonly write these unquoted (e.g.
// `paths: [**/*.ts]`); a leading '*' is a YAML alias indicator, so the whole
// frontmatter fails to parse. sanitizeGlobFrontmatter quotes such values.
var globFrontmatterKeys = map[string]bool{
	"paths":   true,
	"globs":   true,
	"applyto": true, // Copilot's applyTo (compared lower-cased)
}

// parseLenientFrontmatter parses content's frontmatter into v, returning the
// markdown body. It first tries a strict parse; on failure it quotes unquoted
// glob values (the common hand-authored breakage) and retries once. The
// original strict error is returned when nothing could be sanitized.
func parseLenientFrontmatter(content string, v any) ([]byte, error) {
	body, err := frontmatter.Parse(strings.NewReader(content), v)
	if err == nil {
		return body, nil
	}
	sanitized, changed := sanitizeGlobFrontmatter(content)
	if !changed {
		return nil, err
	}
	return frontmatter.Parse(strings.NewReader(sanitized), v)
}

// sanitizeGlobFrontmatter rewrites the YAML frontmatter block so glob values
// under globFrontmatterKeys parse as plain strings, handling flow sequences
// (`[**/*.ts, **/*.tsx]`), block sequences, and bare scalars. Brace-internal
// commas (`{ts,tsx}`) are preserved. Returns the rewritten content and whether
// anything changed. Non-YAML or unfenced content is returned unchanged.
func sanitizeGlobFrontmatter(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return content, false
	}

	changed := false
	inGlobBlock := false // currently inside a glob key's block sequence
	for i := 1; i < end; i++ {
		raw := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimLeft(raw, " \t")
		indent := raw[:len(raw)-len(trimmed)]

		// block-sequence item under a glob key
		if inGlobBlock && strings.HasPrefix(trimmed, "- ") {
			if q, ok := quoteGlobScalar(strings.TrimSpace(trimmed[2:])); ok {
				lines[i] = indent + "- " + q
				changed = true
			}
			continue
		}

		key, rest, isKV := splitYAMLKey(trimmed)
		if !isKV {
			inGlobBlock = false
			continue
		}
		inGlobBlock = false
		if !globFrontmatterKeys[strings.ToLower(key)] {
			continue
		}

		value := strings.TrimSpace(rest)
		switch {
		case value == "":
			inGlobBlock = true // block sequence follows on next lines
		case strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"):
			inner := value[1 : len(value)-1]
			if strings.TrimSpace(inner) == "" {
				continue
			}
			items := splitFlowItems(inner)
			rowChanged := false
			for j, it := range items {
				q, ok := quoteGlobScalar(strings.TrimSpace(it))
				items[j] = q
				rowChanged = rowChanged || ok
			}
			if rowChanged {
				lines[i] = indent + key + ": [" + strings.Join(items, ", ") + "]"
				changed = true
			}
		default:
			if q, ok := quoteGlobScalar(value); ok {
				lines[i] = indent + key + ": " + q
				changed = true
			}
		}
	}

	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

// quoteGlobScalar single-quotes s when it begins with a YAML indicator that
// breaks a plain scalar (notably '*', read as an alias). Already-quoted or
// safe values are returned unchanged with ok=false.
func quoteGlobScalar(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	if s[0] == '\'' || s[0] == '"' {
		return s, false
	}
	if !strings.ContainsRune("*&!?@%`", rune(s[0])) {
		return s, false
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'", true
}

// splitFlowItems splits a YAML flow-sequence inner string on top-level commas,
// leaving commas inside brace/bracket groups (e.g. glob `{ts,tsx}`) intact.
func splitFlowItems(s string) []string {
	var items []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				items = append(items, s[start:i])
				start = i + 1
			}
		}
	}
	return append(items, s[start:])
}

// splitYAMLKey splits a trimmed line "key: rest" into its plain-scalar key and
// the remainder. Reports false for comments, non-mappings, quoted/spaced keys,
// or a value not separated from the colon by whitespace.
func splitYAMLKey(line string) (key, rest string, ok bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = line[:idx]
	rest = line[idx+1:]
	if key == "" || strings.ContainsAny(key, " \t'\"") {
		return "", "", false
	}
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return "", "", false
	}
	return key, rest, true
}
