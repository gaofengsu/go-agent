package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sanbuphy/go-agent/pkg/llm"
	"github.com/sanbuphy/go-agent/pkg/skills"
	"github.com/sanbuphy/go-agent/pkg/tools"
)

func main() {
	skillsDir := "./skills-fake"
	args := os.Args[1:]
	for i, a := range args {
		if a == "--skills-dir" && i+1 < len(args) {
			skillsDir = args[i+1]
			args = append(args[:i], args[i+2:]...)
			break
		}
	}
	if len(args) < 1 {
		fmt.Println("Usage: go run ./cmd/skills [--skills-dir dir] 'your task'")
		os.Exit(1)
	}
	task := strings.Join(args, " ")
	client := llm.NewOpenAIClient()
	store := skills.NewStore(skillsDir)
	discovered, _ := store.Discover()

	registry := tools.NewBaseRegistry()
	if len(discovered) > 0 {
		schema := store.BuildActivateToolSchema()
		if schema.Function.Name != "" {
			registry.Register(&activateTool{store: store, schema: schema})
		}
	}

	skillList := ""
	if len(discovered) > 0 {
		var lines []string
		for _, s := range discovered {
			lines = append(lines, fmt.Sprintf("- %s: %s", s.Name, s.Description))
		}
		skillList = "\n\nAvailable skills:\n" + strings.Join(lines, "\n") + "\n\nUse activate_skill when needed."
	}
	systemPrompt := "You are a helpful assistant. Be concise." + skillList

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: task},
	}

	for i := 0; i < 10; i++ {
		resp, err := client.Chat(context.Background(), msgs, registry.BuildSchemas())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		msgs = append(msgs, llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		if len(resp.ToolCalls) == 0 {
			fmt.Println(resp.Content)
			return
		}
		for _, tc := range resp.ToolCalls {
			name := tc.Function.Name
			args := tc.Function.Arguments
			var result string
			if name == "activate_skill" {
				var a struct{ Name string `json:"name"` }
				fmt.Sscanf(args, `{"name":"%s"}`, &a.Name)
				// simple parse
				if idx := strings.Index(args, `"name"`); idx > 0 {
					start := strings.Index(args[idx:], `"`)
					if start > 0 {
						start += idx + 1
						end := strings.Index(args[start:], `"`)
						if end > 0 {
							a.Name = args[start : start+end]
						}
					}
				}
				fmt.Printf("[Activating] %s\n", a.Name)
				result = store.Activate(a.Name)
			} else {
				res, err := registry.Execute(context.Background(), name, args)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				} else {
					result = res
				}
				fmt.Printf("[Tool] %s(%s)\n", name, args)
			}
			msgs = append(msgs, llm.Message{Role: llm.RoleTool, Content: result, ToolCallID: tc.ID})
		}
	}
	fmt.Println("Max iterations reached")
}

type activateTool struct {
	store  *skills.Store
	schema llm.ToolSchema
}

func (t *activateTool) Name() string        { return "activate_skill" }
func (t *activateTool) Description() string { return "Activate a specialized skill" }
func (t *activateTool) Schema() llm.ToolSchema { return t.schema }
func (t *activateTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	return t.store.Activate(name), nil
}
