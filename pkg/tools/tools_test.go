package tools

import (
	"context"
	"testing"
)

func TestRegistryExecute(t *testing.T) {
	r := NewBaseRegistry()
	// bash echo
	res, err := r.Execute(context.Background(), "execute_bash", `{"command":"echo hello"}`)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res != "hello\n" {
		t.Errorf("unexpected output: %q", res)
	}
}

func TestRegistryUnknownTool(t *testing.T) {
	r := NewBaseRegistry()
	_, err := r.Execute(context.Background(), "unknown_tool", `{}`)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistryInvalidJSON(t *testing.T) {
	r := NewBaseRegistry()
	_, err := r.Execute(context.Background(), "execute_bash", `not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFileTools(t *testing.T) {
	ctx := context.Background()
	write := &WriteFileTool{}
	res, err := write.Execute(ctx, map[string]any{"path": "/tmp/go_agent_test.txt", "content": "test data"})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if res == "" {
		t.Error("expected non-empty result")
	}

	read := &ReadFileTool{}
	res, err = read.Execute(ctx, map[string]any{"path": "/tmp/go_agent_test.txt"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if res != "test data" {
		t.Errorf("read mismatch: %q", res)
	}
}
