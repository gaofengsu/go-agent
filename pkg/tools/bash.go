package tools

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

// BashTool executes shell commands.
type BashTool struct{}

func (t *BashTool) Name() string        { return "execute_bash" }
func (t *BashTool) Description() string { return "Execute a bash command" }

func (t *BashTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`),
		},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	cmdStr, _ := args["command"].(string)
	if cmdStr == "" {
		return "", fmt.Errorf("missing command")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out) + "\n" + err.Error(), nil
	}
	return string(out), nil
}

// BashTool2 is the claudecode variant.
type BashTool2 struct{}

func (t *BashTool2) Name() string        { return "bash" }
func (t *BashTool2) Description() string { return "Run shell command" }

func (t *BashTool2) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`),
		},
	}
}

func (t *BashTool2) Execute(ctx context.Context, args map[string]any) (string, error) {
	return (&BashTool{}).Execute(ctx, args)
}
