package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanbuphy/go-agent/pkg/llm"
	"github.com/sanbuphy/go-agent/pkg/skills"
	"github.com/sanbuphy/go-agent/pkg/tools"
)

// ClaudecodeAgent is a feature-rich agent inspired by Claude Code.
type ClaudecodeAgent struct {
	*Agent
	skillsStore *skills.Store
	planMode    bool
	currentPlan []string
}

// NewClaudecodeAgent creates the advanced agent.
func NewClaudecodeAgent(client llm.LLMClient) *ClaudecodeAgent {
	return &ClaudecodeAgent{
		Agent: &Agent{
			Client:        client,
			Registry:      tools.NewRegistry(client),
			MaxIterations: 5,
		},
	}
}

// Run executes a task with optional planning.
func (a *ClaudecodeAgent) Run(ctx context.Context, task string, usePlan bool) (string, error) {
	fmt.Println("[Init] Loading ClaudeCode features...")
	memory := loadMemory()
	rules := loadRules()
	skillList := a.loadSkills()
	mcpTools := loadMCPTools()

	contextParts := []string{"You are a helpful assistant that can interact with the system. Be concise."}
	if rules != "" {
		contextParts = append(contextParts, "# Rules\n"+rules)
		fmt.Printf("[Rules] Loaded\n")
	}
	if len(skillList) > 0 {
		var lines []string
		for _, s := range skillList {
			lines = append(lines, fmt.Sprintf("- %s: %s", s.Name, s.Description))
		}
		contextParts = append(contextParts, "# Skills\n"+strings.Join(lines, "\n"))
		fmt.Printf("[Skills] Loaded %d skills\n", len(skillList))
	}
	if len(mcpTools) > 0 {
		fmt.Printf("[MCP] Loaded %d MCP tools\n", len(mcpTools))
	}
	if memory != "" {
		contextParts = append(contextParts, "# Previous Context\n"+memory)
	}

	allTools := append(a.Registry.BuildSchemas(), mcpTools...)
	messages := []llm.Message{{Role: llm.RoleSystem, Content: strings.Join(contextParts, "\n\n")}}

	var finalResult string
	if usePlan {
		steps, err := tools.CreatePlan(ctx, a.Client, task)
		if err != nil {
			steps = []string{task}
		}
		a.currentPlan = steps
		fmt.Printf("[Plan] Created %d steps\n", len(steps))
		var results []string
		for i, step := range steps {
			fmt.Printf("\n[Step %d/%d] %s\n", i+1, len(steps), step)
			messages = append(messages, llm.Message{Role: llm.RoleUser, Content: step})
			result, msgs, err := a.runClaudecodeStep(ctx, messages, allTools)
			if err != nil {
				return "", err
			}
			messages = msgs
			results = append(results, result)
			fmt.Printf("\n%s\n", result)
		}
		a.currentPlan = nil
		finalResult = strings.Join(results, "\n")
	} else {
		messages = append(messages, llm.Message{Role: llm.RoleUser, Content: task})
		result, _, err := a.runClaudecodeStep(ctx, messages, allTools)
		if err != nil {
			return "", err
		}
		finalResult = result
		fmt.Printf("\n%s\n", result)
	}
	saveMemory(task, finalResult)
	return finalResult, nil
}

func (a *ClaudecodeAgent) runClaudecodeStep(ctx context.Context, messages []llm.Message, toolSchemas []llm.ToolSchema) (string, []llm.Message, error) {
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
			name := tc.Function.Name
			args := tc.Function.Arguments
			fmt.Printf("[Tool] %s(%s)\n", name, args)
			var result string
			if name == "plan" {
				if a.planMode {
					result = "Error: Cannot plan within a plan"
				} else {
					a.planMode = true
					var planArgs struct{ Task string `json:"task"` }
					json.Unmarshal([]byte(args), &planArgs)
					steps, _ := tools.CreatePlan(ctx, a.Client, planArgs.Task)
					a.currentPlan = steps
					result = fmt.Sprintf("Plan created with %d steps. Executing now...", len(steps))
					messages = append(messages, llm.Message{Role: llm.RoleTool, Content: result, ToolCallID: tc.ID})
					if len(a.currentPlan) > 0 {
						var subResults []string
						for j, step := range a.currentPlan {
							fmt.Printf("\n[Step %d/%d] %s\n", j+1, len(a.currentPlan), step)
							messages = append(messages, llm.Message{Role: llm.RoleUser, Content: step})
							subRes, msgs, _ := a.runClaudecodeStep(ctx, messages, a.Registry.BuildSchemasExcept("plan"))
							messages = msgs
							subResults = append(subResults, subRes)
							fmt.Printf("\n%s\n", subRes)
						}
						a.planMode = false
						a.currentPlan = nil
						return strings.Join(subResults, "\n"), messages, nil
					}
				}
			} else if name == "activate_skill" && a.skillsStore != nil {
				var skillArgs struct{ Name string `json:"name"` }
				json.Unmarshal([]byte(args), &skillArgs)
				fmt.Printf("[Activating] %s\n", skillArgs.Name)
				result = a.skillsStore.Activate(skillArgs.Name)
			} else {
				res, err := a.Registry.Execute(ctx, name, args)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				} else {
					result = res
				}
			}
			messages = append(messages, llm.Message{Role: llm.RoleTool, Content: result, ToolCallID: tc.ID})
		}
	}
	return "Max iterations reached", messages, nil
}

func (a *ClaudecodeAgent) loadSkills() []skills.Skill {
	s := skills.NewStore("./skills-real")
	sk, _ := s.Discover()
	if len(sk) > 0 {
		schema := s.BuildActivateToolSchema()
		if schema.Function.Name != "" {
			a.Registry.Register(&activateSkillTool{store: s, schema: schema})
		}
	}
	a.skillsStore = s
	return sk
}

type activateSkillTool struct {
	store  *skills.Store
	schema llm.ToolSchema
}

func (t *activateSkillTool) Name() string        { return "activate_skill" }
func (t *activateSkillTool) Description() string { return "Activate a specialized skill" }
func (t *activateSkillTool) Schema() llm.ToolSchema { return t.schema }
func (t *activateSkillTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	return t.store.Activate(name), nil
}

func loadRules() string {
	dir := ".agent/rules"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			if len(b) > 0 {
				parts = append(parts, fmt.Sprintf("# %s\n%s", strings.TrimSuffix(e.Name(), ".md"), string(b)))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func loadMCPTools() []llm.ToolSchema {
	b, err := os.ReadFile(".agent/mcp.json")
	if err != nil {
		return nil
	}
	var cfg struct {
		MCPServers map[string]struct {
			Disabled bool              `json:"disabled"`
			Tools    []llm.FunctionSchema `json:"tools"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil
	}
	var out []llm.ToolSchema
	for _, srv := range cfg.MCPServers {
		if srv.Disabled {
			continue
		}
		for _, t := range srv.Tools {
			out = append(out, llm.ToolSchema{Type: "function", Function: t})
		}
	}
	return out
}

func loadMemory() string {
	b, err := os.ReadFile(memoryFile)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}
	return strings.Join(lines, "\n")
}

func saveMemory(task, result string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("\n## %s\n**Task:** %s\n**Result:** %s\n", ts, task, result)
	f, err := os.OpenFile(memoryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}
