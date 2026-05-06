package testutil

import (
	"context"
	"errors"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

// MockClient is a test double for LLMClient.
type MockClient struct {
	Responses []llm.ChatResponse
	Index     int
	Embeds    [][]float32
}

func (m *MockClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolSchema) (*llm.ChatResponse, error) {
	if m.Index >= len(m.Responses) {
		return nil, errors.New("no more mock responses")
	}
	resp := &m.Responses[m.Index]
	m.Index++
	return resp, nil
}

func (m *MockClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.Embeds != nil {
		return m.Embeds, nil
	}
	// return zero embeddings
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = make([]float32, 1536)
	}
	return out, nil
}

// NewMockClient creates a mock with a single text response.
func NewMockClient(content string) *MockClient {
	return &MockClient{Responses: []llm.ChatResponse{{Content: content}}}
}

// NewMockToolClient creates a mock that first requests a tool then replies.
func NewMockToolClient(toolName, toolArgs, finalContent string) *MockClient {
	return &MockClient{Responses: []llm.ChatResponse{
		{ToolCalls: []llm.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      toolName,
				Arguments: toolArgs,
			},
		}}},
		{Content: finalContent},
	}}
}
