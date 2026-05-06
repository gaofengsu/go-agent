package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/agent"
	"github.com/sanbuphy/go-agent/pkg/llm"
)

func main() {
	usePlan := false
	args := os.Args[1:]
	for i, a := range args {
		if a == "--plan" {
			usePlan = true
			args = append(args[:i], args[i+1:]...)
			break
		}
	}
	if len(args) < 1 {
		fmt.Println("Usage: go run ./cmd/agent_plus [--plan] 'your task here'")
		fmt.Println("  --plan: Enable task planning and decomposition")
		os.Exit(1)
	}
	task := strings.Join(args, " ")
	client := llm.NewOpenAIClient()
	a := agent.NewPlusAgent(client)
	result, err := a.RunWithPlan(context.Background(), task, usePlan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}
