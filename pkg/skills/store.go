package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

// Store manages skill discovery and activation.
type Store struct {
	Dir    string
	skills []Skill
}

// NewStore creates a skill store for the given directory.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Discover scans for SKILL.md files recursively.
func (s *Store) Discover() ([]Skill, error) {
	if _, err := os.Stat(s.Dir); os.IsNotExist(err) {
		return nil, nil
	}
	var discovered []Skill
	_ = filepath.Walk(s.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "SKILL.md") {
			skill, err := ParseSkill(path)
			if err == nil {
				discovered = append(discovered, *skill)
			}
		}
		return nil
	})
	s.skills = discovered
	return discovered, nil
}

// Activate returns the XML-wrapped skill body.
func (s *Store) Activate(name string) string {
	for _, sk := range s.skills {
		if sk.Name == name {
			return fmt.Sprintf("<activated_skill name=\"%s\">\n%s\n</activated_skill>", name, sk.Body)
		}
	}
	return fmt.Sprintf("Error: Skill '%s' not found", name)
}

// BuildActivateToolSchema creates the JSON schema for the activate_skill tool.
func (s *Store) BuildActivateToolSchema() llm.ToolSchema {
	if len(s.skills) == 0 {
		return llm.ToolSchema{}
	}
	var names []string
	for _, sk := range s.skills {
		names = append(names, sk.Name)
	}
	enumJSON := fmt.Sprintf(`["%s"]`, strings.Join(names, `","`))
	params := fmt.Sprintf(`{"type":"object","properties":{"name":{"type":"string","enum":%s}},"required":["name"]}`, enumJSON)
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        "activate_skill",
			Description: "Activate a specialized skill",
			Parameters:  []byte(params),
		},
	}
}
