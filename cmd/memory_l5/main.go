package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/llm"
	"github.com/sanbuphy/go-agent/pkg/memory"
)

func main() {
	task := "Hello"
	if len(os.Args) > 1 {
		task = strings.Join(os.Args[1:], " ")
	}
	client := llm.NewOpenAIClient()
	mem := memory.NewL5SummaryMemory(client)

	relevant, _ := mem.Search(context.Background(), task, 5)
	memoryBlock := ""
	if len(relevant) > 0 {
		var lines []string
		for _, e := range relevant {
			lines = append(lines, "- "+e.Text)
		}
		memoryBlock = "\n\nPast context:\n" + strings.Join(lines, "\n")
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant. Be concise." + memoryBlock},
		{Role: llm.RoleUser, Content: task},
	}
	resp, err := client.Chat(context.Background(), msgs, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp.Content)
	mem.Save(context.Background(), task, resp.Content)
}
