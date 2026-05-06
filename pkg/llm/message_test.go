package llm

import (
	"encoding/json"
	"testing"
)

func TestMessageSerialization(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "hello",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "test",
					Arguments: `{"a":1}`,
				},
			},
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Role != RoleUser {
		t.Errorf("role mismatch: got %s", decoded.Role)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Errorf("toolcalls mismatch: got %d", len(decoded.ToolCalls))
	}
}
