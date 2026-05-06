package skills

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Skill represents a parsed skill file.
type Skill struct {
	Name        string
	Description string
	Body        string
}

var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)

// ParseSkill reads and parses a SKILL.md file.
func ParseSkill(path string) (*Skill, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(b)
	m := frontmatterRe.FindStringSubmatch(content)
	if m == nil {
		return nil, fmt.Errorf("invalid skill format: %s", path)
	}
	frontmatter, body := m[1], strings.TrimSpace(m[2])
	var name, desc string
	for _, line := range strings.Split(frontmatter, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			switch key {
			case "name":
				name = val
			case "description":
				desc = val
			}
		}
	}
	if name == "" || desc == "" {
		return nil, fmt.Errorf("skill missing name or description: %s", path)
	}
	return &Skill{Name: name, Description: desc, Body: body}, nil
}
