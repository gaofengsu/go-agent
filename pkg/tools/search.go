package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

// GlobTool finds files by pattern.
type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Find files by pattern" }

func (t *GlobTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"pattern":{"type":"string"}},"required":["pattern"]}`),
		},
	}
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("invalid pattern argument")
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	// also try recursive
	if len(matches) == 0 && strings.Contains(pattern, "**") {
		root := "."
		if idx := strings.Index(pattern, "/**"); idx > 0 {
			root = pattern[:idx]
			pattern = pattern[idx+1:]
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if matched, _ := filepath.Match(pattern, path); matched {
				matches = append(matches, path)
			}
			return nil
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi != nil && fj != nil {
			return fi.ModTime().After(fj.ModTime())
		}
		return matches[i] < matches[j]
	})
	if len(matches) == 0 {
		return "No files found", nil
	}
	return strings.Join(matches, "\n"), nil
}

// GrepTool searches files for a pattern.
type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search files for pattern" }

func (t *GrepTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"}},"required":["pattern"]}`),
		},
	}
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	pattern, ok1 := args["pattern"].(string)
	if !ok1 {
		return "", fmt.Errorf("invalid pattern argument")
	}
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	cmd := exec.CommandContext(ctx, "grep", "-r", pattern, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return string(out), nil
		}
		return "No matches found", nil
	}
	return string(out), nil
}
