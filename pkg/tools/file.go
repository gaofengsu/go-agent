package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

const defaultFilePerm = 0644

// ReadFileTool reads a file (base agent variant).
type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read a file" }

func (t *ReadFileTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
		},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("invalid path argument")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WriteFileTool writes a file (base agent variant).
type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write to a file" }

func (t *WriteFileTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, ok1 := args["path"].(string)
	content, ok2 := args["content"].(string)
	if !ok1 || !ok2 {
		return "", fmt.Errorf("invalid path or content argument")
	}
	if err := os.WriteFile(path, []byte(content), defaultFilePerm); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote to %s", path), nil
}

// ReadTool reads a file with line numbers (claudecode variant).
type ReadTool struct{}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string { return "Read file with line numbers" }

func (t *ReadTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["path"]}`),
		},
	}
}

func (t *ReadTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("invalid path argument")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	offsetF, _ := args["offset"].(float64)
	limitF, _ := args["limit"].(float64)
	offset := int(offsetF)
	limit := int(limitF)
	start := offset
	if start < 0 || start > len(lines) {
		start = 0
	}
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	var out strings.Builder
	for i := start; i < end; i++ {
		out.WriteString(fmt.Sprintf("%4d %s\n", i+1, lines[i]))
	}
	return out.String(), nil
}

// WriteTool writes a file (claudecode variant).
type WriteTool struct{}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Description() string { return "Write content to file" }

func (t *WriteTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		},
	}
}

func (t *WriteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return (&WriteFileTool{}).Execute(ctx, args)
}

// EditTool replaces a unique string in a file.
type EditTool struct{}

func (t *EditTool) Name() string        { return "edit" }
func (t *EditTool) Description() string { return "Replace string in file" }

func (t *EditTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"}},"required":["path","old_string","new_string"]}`),
		},
	}
}

func (t *EditTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, ok1 := args["path"].(string)
	oldStr, ok2 := args["old_string"].(string)
	newStr, ok3 := args["new_string"].(string)
	if !ok1 || !ok2 || !ok3 {
		return "", fmt.Errorf("invalid path, old_string or new_string argument")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(b)
	if strings.Count(content, oldStr) != 1 {
		return "", fmt.Errorf("old_string must appear exactly once")
	}
	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(path, []byte(newContent), defaultFilePerm); err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully edited %s", path), nil
}
