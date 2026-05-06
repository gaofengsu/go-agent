package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// LLMClient defines the interface for chat and embedding operations.
type LLMClient interface {
	Chat(ctx context.Context, messages []Message, tools []ToolSchema) (*ChatResponse, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// OpenAIClient implements LLMClient using go-openai.
type OpenAIClient struct {
	client *openai.Client
	model  string
	embed  string
}

// NewOpenAIClient creates a client from environment or defaults.
func NewOpenAIClient() *OpenAIClient {
	cfg := openai.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	if base := os.Getenv("OPENAI_BASE_URL"); base != "" {
		cfg.BaseURL = base
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	embed := os.Getenv("OPENAI_EMBED_MODEL")
	if embed == "" {
		embed = "text-embedding-3-small"
	}
	return &OpenAIClient{client: openai.NewClientWithConfig(cfg), model: model, embed: embed}
}

// Chat sends messages and optional tools to the LLM.
func (c *OpenAIClient) Chat(ctx context.Context, messages []Message, tools []ToolSchema) (*ChatResponse, error) {
	var oaiMessages []openai.ChatCompletionMessage
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{Role: string(RoleSystem), Content: m.Content})
		case RoleUser:
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{Role: string(RoleUser), Content: m.Content})
		case RoleAssistant:
			msg := openai.ChatCompletionMessage{Role: string(RoleAssistant), Content: m.Content}
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
			oaiMessages = append(oaiMessages, msg)
		case RoleTool:
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
				Role:       string(RoleTool),
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
			})
		}
	}

	var oaiTools []openai.Tool
	for _, t := range tools {
		oaiTools = append(oaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: oaiMessages,
	}
	if len(oaiTools) > 0 {
		req.Tools = oaiTools
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	choice := resp.Choices[0].Message
	var tcs []ToolCall
	for _, tc := range choice.ToolCalls {
		tcs = append(tcs, ToolCall{
			ID:   tc.ID,
			Type: string(tc.Type),
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return &ChatResponse{Content: choice.Content, ToolCalls: tcs}, nil
}

// Embed returns embeddings for the provided texts.
func (c *OpenAIClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	req := openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.EmbeddingModel(c.embed),
	}
	resp, err := c.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}
	var out [][]float32
	for _, d := range resp.Data {
		var v []float32
		for _, f := range d.Embedding {
			v = append(v, float32(f))
		}
		out = append(out, v)
	}
	return out, nil
}
