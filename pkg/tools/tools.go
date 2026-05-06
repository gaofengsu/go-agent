package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

// Tool is a callable capability exposed to the LLM.
type Tool interface {
	Name() string
	Description() string
	Schema() llm.ToolSchema
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds registered tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a registry with claudecode-style rich tools.
func NewRegistry(client llm.LLMClient) *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.Register(&BashTool2{})
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&GlobTool{})
	r.Register(&GrepTool{})
	r.Register(&PlanTool{Client: client})
	return r
}

// NewBaseRegistry creates a registry with only base tools (no plan).
func NewBaseRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.Register(&BashTool{})
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	return r
}

// Register adds a tool.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Execute runs a tool by name with JSON-decoded arguments.
func (r *Registry) Execute(ctx context.Context, name string, arguments string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	var args map[string]any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid JSON arguments: %w", err)
		}
	}
	return t.Execute(ctx, args)
}

// BuildSchemas returns tool schemas for all registered tools.
func (r *Registry) BuildSchemas() []llm.ToolSchema {
	var schemas []llm.ToolSchema
	for _, t := range r.tools {
		schemas = append(schemas, t.Schema())
	}
	return schemas
}

// BuildSchemasExcept returns schemas excluding the named tools.
func (r *Registry) BuildSchemasExcept(exclude ...string) []llm.ToolSchema {
	excludeSet := make(map[string]struct{})
	for _, n := range exclude {
		excludeSet[n] = struct{}{}
	}
	var schemas []llm.ToolSchema
	for name, t := range r.tools {
		if _, ok := excludeSet[name]; ok {
			continue
		}
		schemas = append(schemas, t.Schema())
	}
	return schemas
}
