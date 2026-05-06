package agent

import (
	"context"
	"fmt"

	"github.com/sanbuphy/go-agent/pkg/llm"
	"github.com/sanbuphy/go-agent/pkg/tools"
)

// Agent is the base agent with tool-calling loop.
type Agent struct {
	Client        llm.LLMClient
	Registry      *tools.Registry
	MaxIterations int
}

// NewAgent creates a base agent.
func NewAgent(client llm.LLMClient) *Agent {
	return &Agent{
		Client:        client,
		Registry:      tools.NewBaseRegistry(),
		MaxIterations: 5,
	}
}

// Run executes the agent loop for a single user message.
func (a *Agent) Run(ctx context.Context, systemPrompt, userMessage string) (string, []llm.Message, error) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: userMessage},
	}
	result, msgs, err := a.runStep(ctx, messages, a.Registry.BuildSchemas())
	return result, msgs, err
}

func (a *Agent) runStep(ctx context.Context, messages []llm.Message, toolSchemas []llm.ToolSchema) (string, []llm.Message, error) {
	for i := 0; i < a.MaxIterations; i++ {
		resp, err := a.Client.Chat(ctx, messages, toolSchemas)
		if err != nil {
			return "", messages, err
		}
		msg := llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls}
		messages = append(messages, msg)
		if len(resp.ToolCalls) == 0 {
			return resp.Content, messages, nil
		}
		for _, tc := range resp.ToolCalls {
			fmt.Printf("[Tool] %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			result, err := a.Registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %s", err.Error())
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}
	return "Max iterations reached", messages, nil
}
