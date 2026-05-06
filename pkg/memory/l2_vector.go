package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

const l2File = "memory_l2.jsonl"

// L2VectorMemory uses OpenAI embeddings + cosine similarity.
type L2VectorMemory struct {
	Client llm.LLMClient
	Model  string
}

func NewL2VectorMemory(client llm.LLMClient) *L2VectorMemory {
	return &L2VectorMemory{Client: client, Model: os.Getenv("OPENAI_MODEL")}
}

func (m *L2VectorMemory) Save(ctx context.Context, userInput, aiResponse string) error {
	if aiResponse == "" || m.Client == nil {
		return nil
	}
	facts, err := m.extractFacts(ctx, userInput, aiResponse)
	if err != nil {
		return err
	}
	for _, f := range facts {
		emb, err := m.Client.Embed(ctx, []string{f})
		if err != nil {
			return err
		}
		entry := map[string]any{
			"text":      f,
			"embedding": emb[0],
			"timestamp": time.Now().Format(time.RFC3339),
			"metadata":  map[string]any{},
		}
		if err := appendJSONL(l2File, entry); err != nil {
			return err
		}
		fmt.Printf("[Memory] Saved: %s\n", f)
	}
	return nil
}

func (m *L2VectorMemory) Search(ctx context.Context, query string, topK int) ([]Entry, error) {
	if m.Client == nil {
		return nil, nil
	}
	queryEmb, err := m.Client.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	memories, err := loadJSONL(l2File)
	if err != nil {
		return nil, err
	}
	type scored struct {
		entry map[string]any
		score float32
	}
	var scoredList []scored
	for _, mem := range memories {
		embRaw, ok := mem["embedding"].([]any)
		if !ok {
			continue
		}
		emb := make([]float32, len(embRaw))
		for i, v := range embRaw {
			if f, ok := v.(float64); ok {
				emb[i] = float32(f)
			}
		}
		sim := cosineSimilarity(queryEmb[0], emb)
		scoredList = append(scoredList, scored{entry: mem, score: sim})
	}
	// sort desc
	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[j].score > scoredList[i].score {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}
	var results []Entry
	for i, s := range scoredList {
		if i >= topK {
			break
		}
		e := mapToEntry(s.entry)
		e.Score = s.score
		results = append(results, e)
	}
	return results, nil
}

func (m *L2VectorMemory) extractFacts(ctx context.Context, userInput, aiResponse string) ([]string, error) {
	if m.Client == nil {
		return nil, nil
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Extract key facts worth remembering from this conversation.
Ignore greetings and small talk. Only facts, preferences, and decisions.

User: %s
AI: %s

Return JSON array of strings. If nothing worth remembering, return [].`, userInput, aiResponse)},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return nil, err
	}
	var facts []string
	if err := json.Unmarshal([]byte(resp.Content), &facts); err != nil {
		return nil, nil
	}
	return facts, nil
}

func cosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	return dot / (float32(math.Sqrt(float64(normA*normB))) + 1e-8)
}
