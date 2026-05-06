package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/sanbuphy/go-agent/pkg/llm"
)

const (
	l3File         = "memory_l3.jsonl"
	reflectionFile = "memory_reflections.jsonl"
	alpha          = 0.5
	beta           = 0.3
	gamma          = 0.2
	halfLifeDays   = 30.0
)

// L3ScoredMemory uses hybrid scoring + reflection.
type L3ScoredMemory struct {
	Client llm.LLMClient
	Model  string
}

func NewL3ScoredMemory(client llm.LLMClient) *L3ScoredMemory {
	return &L3ScoredMemory{Client: client, Model: os.Getenv("OPENAI_MODEL")}
}

func (m *L3ScoredMemory) Save(ctx context.Context, userInput, aiResponse string) error {
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
		imp := m.scoreImportance(ctx, f)
		entry := map[string]any{
			"text":         f,
			"embedding":    emb[0],
			"importance":   imp,
			"timestamp":    time.Now().Format(time.RFC3339),
			"access_count": 0,
		}
		if err := appendJSONL(l3File, entry); err != nil {
			return err
		}
		fmt.Printf("[Memory] Saved (importance=%d): %s\n", imp, f)
	}
	// trigger reflection if enough memories
	mems, _ := loadJSONL(l3File)
	if len(mems) > 0 && len(mems)%10 == 0 {
		m.reflect(ctx, mems)
	}
	return nil
}

func (m *L3ScoredMemory) Search(ctx context.Context, query string, topK int) ([]Entry, error) {
	if m.Client == nil {
		return nil, nil
	}
	queryEmb, err := m.Client.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	memories, err := loadJSONL(l3File)
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
		tsStr, _ := mem["timestamp"].(string)
		ts, _ := time.Parse(time.RFC3339, tsStr)
		rec := recencyScore(ts)
		impF, _ := mem["importance"].(float64)
		imp := float32(impF) / 10.0
		score := alpha*sim + beta*rec + gamma*imp
		scoredList = append(scoredList, scored{entry: mem, score: score})
		mem["access_count"] = int(mem["access_count"].(float64)) + 1
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

func (m *L3ScoredMemory) scoreImportance(ctx context.Context, text string) int {
	if m.Client == nil {
		return 5
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Rate the importance of this memory on a scale of 1-10.
10 = critical fact/preference, 1 = trivial detail.
Only output a single number.

Memory: %s`, text)},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return 5
	}
	v, _ := strconv.Atoi(resp.Content)
	if v < 1 {
		v = 1
	}
	if v > 10 {
		v = 10
	}
	return v
}

func (m *L3ScoredMemory) reflect(ctx context.Context, memories []map[string]any) {
	if len(memories) < 3 || m.Client == nil {
		return
	}
	var texts []string
	start := 0
	if len(memories) > 20 {
		start = len(memories) - 20
	}
	for _, mem := range memories[start:] {
		texts = append(texts, mem["text"].(string))
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Review these recent memories and generate 2-3 high-level insights.
These should be abstract patterns or lessons learned, not raw facts.

Memories:
%s

Return JSON array of insight strings.`, joinBullet(texts))},
	}
	resp, err := m.Client.Chat(ctx, msgs, nil)
	if err != nil {
		return
	}
	var insights []string
	if err := json.Unmarshal([]byte(resp.Content), &insights); err != nil {
		return
	}
	for _, ins := range insights {
		entry := map[string]any{
			"text":      ins,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		appendJSONL(reflectionFile, entry)
		fmt.Printf("[Reflect] %s\n", ins)
	}
}

func recencyScore(ts time.Time) float32 {
	days := time.Since(ts).Hours() / 24.0
	return float32(math.Pow(0.5, days/halfLifeDays))
}

func joinBullet(strs []string) string {
	var out string
	for _, s := range strs {
		out += "- " + s + "\n"
	}
	return out
}

func (m *L3ScoredMemory) extractFacts(ctx context.Context, userInput, aiResponse string) ([]string, error) {
	if m.Client == nil {
		return nil, nil
	}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: fmt.Sprintf(`Extract key facts worth remembering. Ignore small talk.
Return JSON array of strings or [].

User: %s
AI: %s`, userInput, aiResponse)},
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
