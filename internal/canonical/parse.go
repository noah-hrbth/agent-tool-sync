package canonical

import (
	"strings"

	"github.com/adrg/frontmatter"
)

// ParseSkill parses a full file string (frontmatter + body) into s.
// Loader-set fields (Dir) are preserved; typed frontmatter fields and Body are updated.
func ParseSkill(content string, s *Skill) error {
	dir := s.Dir
	body, err := frontmatter.Parse(strings.NewReader(content), s)
	if err != nil {
		return err
	}
	s.Dir = dir
	s.Body = string(body)
	return nil
}

// ParseAgent parses a full file string (frontmatter + body) into a.
// Loader-set fields (Filename) are preserved; typed frontmatter fields and Body are updated.
func ParseAgent(content string, a *Agent) error {
	filename := a.Filename
	body, err := frontmatter.Parse(strings.NewReader(content), a)
	if err != nil {
		return err
	}
	a.Filename = filename
	a.Body = string(body)
	return nil
}

// ParseCommand parses a full file string (frontmatter + body) into cmd.
// Loader-set fields (Filename) are preserved; typed frontmatter fields and Body are updated.
func ParseCommand(content string, cmd *Command) error {
	filename := cmd.Filename
	body, err := frontmatter.Parse(strings.NewReader(content), cmd)
	if err != nil {
		return err
	}
	cmd.Filename = filename
	cmd.Body = string(body)
	return nil
}

// ParseRule parses a full file string (frontmatter + body) into r.
// Loader-set fields (Filename) are preserved; typed frontmatter fields and Body are updated.
func ParseRule(content string, r *Rule) error {
	filename := r.Filename
	body, err := frontmatter.Parse(strings.NewReader(content), r)
	if err != nil {
		return err
	}
	r.Filename = filename
	r.Body = string(body)
	return nil
}
