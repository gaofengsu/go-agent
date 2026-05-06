package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/agent"
	"github.com/sanbuphy/go-agent/pkg/llm"
)

func main() {
	var (
		plan       = flag.Bool("plan", false, "Enable task planning")
		skillsDir  = flag.String("skills-dir", "", "Skills directory")
		memoryLevel = flag.Int("memory-level", 0, "Memory level (0-5)")
		claudecode = flag.Bool("claudecode", false, "Use ClaudeCode-style agent")
	)
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}
	task := strings.Join(args, " ")
	client := llm.NewOpenAIClient()

	if *claudecode {
		a := agent.NewClaudecodeAgent(client)
		result, err := a.Run(context.Background(), task, *plan)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
		return
	}

	if *plan || *memoryLevel > 0 {
		a := agent.NewPlusAgent(client)
		result, err := a.RunWithPlan(context.Background(), task, *plan)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
		_ = skillsDir
		_ = memoryLevel
		return
	}

	a := agent.NewAgent(client)
	result, _, err := a.Run(context.Background(), "You are a helpful assistant. Be concise.", task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func printUsage() {
	fmt.Println("go-agent - Minimal AI Agent in Go")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go-agent [flags] 'your task'")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -plan            Enable task planning")
	fmt.Println("  -claudecode      Use ClaudeCode-style agent with rich tools")
	fmt.Println("  -skills-dir      Specify skills directory")
	fmt.Println("  -memory-level    Set memory level (0-5)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go-agent 'list all go files'")
	fmt.Println("  go-agent -plan 'refactor the codebase'")
	fmt.Println("  go-agent -claudecode 'read README.md and summarize'")
}
