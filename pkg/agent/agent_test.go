package agent

import (
	"context"
	"testing"

	"github.com/sanbuphy/go-agent/internal/testutil"
	"github.com/sanbuphy/go-agent/pkg/llm"
)

func TestBaseAgentRun(t *testing.T) {
	client := testutil.NewMockClient("Hello, user!")
	a := NewAgent(client)
	result, _, err := a.Run(context.Background(), "Be helpful.", "Say hi")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result != "Hello, user!" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestBaseAgentToolCall(t *testing.T) {
	client := testutil.NewMockToolClient("execute_bash", `{"command":"echo test"}`, "Done")
	a := NewAgent(client)
	result, _, err := a.Run(context.Background(), "Be helpful.", "run echo test")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result != "Done" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestPlusAgentRun(t *testing.T) {
	client := testutil.NewMockClient("Result")
	a := NewPlusAgent(client)
	result, err := a.RunWithPlan(context.Background(), "task", false)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result != "Result" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestMessagesAccumulation(t *testing.T) {
	client := testutil.NewMockClient("ok")
	a := NewAgent(client)
	_, msgs, err := a.Run(context.Background(), "sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 { // system + user + assistant
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("expected system, got %s", msgs[0].Role)
	}
}
