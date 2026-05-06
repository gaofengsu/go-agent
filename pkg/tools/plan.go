package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

// PlanTool breaks down complex tasks into steps.
type PlanTool struct {
	Client llm.LLMClient
}

func (t *PlanTool) Name() string        { return "plan" }
func (t *PlanTool) Description() string { return "Break down complex task into steps and execute sequentially" }

func (t *PlanTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  []byte(`{"type":"object","properties":{"task":{"type":"string"}},"required":["task"]}`),
		},
	}
}

func (t *PlanTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	task, _ := args["task"].(string)
	if task == "" {
		return "", fmt.Errorf("missing task")
	}
	// The actual plan execution is handled by the agent loop,
	// so this tool just returns a marker. The agent intercepts it.
	return fmt.Sprintf("Plan requested for: %s", task), nil
}

// CreatePlan asks the LLM to break a task into steps.
func CreatePlan(ctx context.Context, client llm.LLMClient, task string) ([]string, error) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "Break down the task into 3-5 simple, actionable steps. Return as JSON array of strings under key 'steps'."},
		{Role: llm.RoleUser, Content: fmt.Sprintf("Task: %s", task)},
	}
	resp, err := client.Chat(ctx, msgs, nil)
	if err != nil {
		return nil, err
	}
	var data struct {
		Steps []string `json:"steps"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &data); err == nil && len(data.Steps) > 0 {
		return data.Steps, nil
	}
	// try raw array
	var steps []string
	if err := json.Unmarshal([]byte(resp.Content), &steps); err == nil && len(steps) > 0 {
		return steps, nil
	}
	return []string{task}, nil
}
