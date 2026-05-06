package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/llm"
	"github.com/sanbuphy/go-agent/pkg/tools"
)

const memoryFile = "agent_memory.md"

// PlusAgent extends base Agent with memory and planning.
type PlusAgent struct {
	*Agent
}

// NewPlusAgent creates an enhanced agent.
func NewPlusAgent(client llm.LLMClient) *PlusAgent {
	return &PlusAgent{Agent: NewAgent(client)}
}

// RunWithPlan executes with optional task decomposition.
func (a *PlusAgent) RunWithPlan(ctx context.Context, task string, usePlan bool) (string, error) {
	memory := loadMemory()
	system := "You are a helpful assistant that can interact with the system. Be concise."
	if memory != "" {
		system += "\n\nPrevious context:\n" + memory
	}
	messages := []llm.Message{{Role: llm.RoleSystem, Content: system}}

	steps := []string{task}
	if usePlan {
		planSteps, err := tools.CreatePlan(ctx, a.Client, task)
		if err != nil {
			fmt.Printf("[Planning] failed: %v\n", err)
		} else {
			steps = planSteps
			fmt.Printf("[Plan] %d steps created\n", len(steps))
			for i, s := range steps {
				fmt.Printf("  %d. %s\n", i+1, s)
			}
		}
	}

	var results []string
	for i, step := range steps {
		if len(steps) > 1 {
			fmt.Printf("\n[Step %d/%d] %s\n", i+1, len(steps), step)
		}
		messages = append(messages, llm.Message{Role: llm.RoleUser, Content: step})
		result, msgs, err := a.runStep(ctx, messages, a.Registry.BuildSchemas())
		if err != nil {
			return "", err
		}
		messages = msgs
		results = append(results, result)
		fmt.Printf("\n%s\n", result)
	}
	final := strings.Join(results, "\n")
	saveMemory(task, final)
	return final, nil
}


