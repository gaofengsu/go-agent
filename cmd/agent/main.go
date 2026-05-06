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
	task := "Hello"
	if len(os.Args) > 1 {
		task = strings.Join(os.Args[1:], " ")
	}
	client := llm.NewOpenAIClient()
	a := agent.NewAgent(client)
	result, _, err := a.Run(context.Background(), "You are a helpful assistant. Be concise.", task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}
